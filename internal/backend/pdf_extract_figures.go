package backend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"zotero_cli/internal/domain"
)

// FigureInfo describes a single extracted figure.
type FigureInfo struct {
	ID            int     `json:"id"`
	File          string  `json:"file"`
	Page          int     `json:"page"`
	Source        string  `json:"source"` // "vector" | "raster"
	SizePx        string  `json:"size_px"`
	SizePt        string  `json:"size_pt"`
	KB            float64 `json:"kb"`
	Anchors       int     `json:"anchors"`
	Caption       string  `json:"caption,omitempty"`
	HasCaption    bool    `json:"has_caption"`
	TextRatio     float64 `json:"text_ratio"`
	PctPage       float64 `json:"pct_page"`
	Confidence    int     `json:"confidence"`     // V4: 0-100 quality score
	AttachmentKey string  `json:"attachment_key"` // which PDF this figure came from
}

// ExtractFiguresResult is the result of extracting figures from a PDF item.
type ExtractFiguresResult struct {
	ItemKey    string       `json:"item_key"`
	PDFPath    string       `json:"pdf,omitempty"`
	TotalPages int          `json:"total_pages"`
	Figures    []FigureInfo `json:"figures"`
	ElapsedSec float64      `json:"elapsed_sec"`
	Method     string       `json:"method"`
	Error      string       `json:"error,omitempty"`
}

// figureScriptVersion is bumped when the Python script changes, invalidating old caches.
const figureScriptVersion = "v13"

// figureCacheMeta is the persisted manifest for one attachment's extracted figures.
type figureCacheMeta struct {
	AttachmentKey   string       `json:"attachment_key"`
	ResolvedPath    string       `json:"resolved_path"`
	SourceMtimeUnix int64        `json:"source_mtime_unix"`
	SourceSize      int64        `json:"source_size"`
	ScriptVersion   string       `json:"script_version"`
	ExtractedAt     string       `json:"extracted_at"`
	ElapsedSec      float64      `json:"elapsed_sec"`
	TotalPages      int          `json:"total_pages"`
	Figures         []FigureInfo `json:"figures"`
}

// figureCache provides disk-based caching for figure extraction results.
// Layout mirrors fulltext_cache: {rootDir}/{attachmentKey}/manifest.json
type figureCache struct {
	rootDir string
}

func newFigureCache(dataDir string) figureCache {
	return figureCache{rootDir: filepath.Join(dataDir, ".zotero_cli", "figures_cache")}
}

func (c figureCache) metaPath(attKey string) string {
	return filepath.Join(c.rootDir, strings.TrimSpace(attKey), "manifest.json")
}

// Load reads the cached manifest for an attachment. Returns (meta, true) on hit, (zero, false) on miss/error.
func (c figureCache) Load(att domain.Attachment) (figureCacheMeta, bool) {
	key := strings.TrimSpace(att.Key)
	if key == "" {
		return figureCacheMeta{}, false
	}
	data, err := os.ReadFile(c.metaPath(key))
	if err != nil {
		return figureCacheMeta{}, false
	}
	var meta figureCacheMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return figureCacheMeta{}, false
	}
	return meta, true
}

// Save persists a successful extraction result to the cache.
func (c figureCache) Save(att domain.Attachment, result ExtractFiguresResult) error {
	key := strings.TrimSpace(att.Key)
	if key == "" {
		return nil
	}
	dir := filepath.Dir(c.metaPath(key))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	srcInfo, srcOk := attSourceInfo(att)
	meta := figureCacheMeta{
		AttachmentKey:   key,
		ResolvedPath:    att.ResolvedPath,
		SourceMtimeUnix: srcInfo.ModTime().Unix(),
		SourceSize:      srcInfo.Size(),
		ScriptVersion:   figureScriptVersion,
		ExtractedAt:     time.Now().UTC().Format(time.RFC3339),
		ElapsedSec:      result.ElapsedSec,
		TotalPages:      result.TotalPages,
		Figures:         result.Figures,
	}
	if !srcOk {
		meta.SourceMtimeUnix = 0
		meta.SourceSize = 0
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.metaPath(key), data, 0o644)
}

// IsFresh checks whether a cached result is still valid for the given attachment.
func (c figureCache) IsFresh(meta figureCacheMeta, att domain.Attachment) bool {
	key := strings.TrimSpace(att.Key)
	if key == "" || meta.AttachmentKey != key {
		return false
	}
	if meta.ScriptVersion != figureScriptVersion {
		return false
	}
	info, ok := attSourceInfo(att)
	if !ok {
		return false
	}
	if filepath.Clean(meta.ResolvedPath) != filepath.Clean(att.ResolvedPath) {
		return false
	}
	return meta.SourceMtimeUnix == info.ModTime().Unix() && meta.SourceSize == info.Size()
}

func cachedFigureFilesAvailable(meta figureCacheMeta, attDir string) bool {
	for _, fig := range meta.Figures {
		if strings.TrimSpace(fig.File) == "" {
			return false
		}
		if _, err := os.Stat(filepath.Join(attDir, fig.File)); err != nil {
			return false
		}
	}
	return true
}

func pythonJSONPayload(stdout []byte) ([]byte, error) {
	start := bytes.IndexByte(stdout, '{')
	end := bytes.LastIndexByte(stdout, '}')
	if start < 0 || end < start {
		return nil, fmt.Errorf("no JSON object in stdout")
	}
	return stdout[start : end+1], nil
}

// pythonCountPagesScript is a minimal PyMuPDF script that returns page counts for given PDFs.
const pythonCountPagesScript = `
import json, sys, fitz
paths = json.loads(sys.argv[1])
out = {}
for p in paths:
    try:
        doc = fitz.open(p)
        out[p] = doc.page_count
        doc.close()
    except Exception as e:
        out[p] = -1
print(json.dumps(out))
`

// CountPDFPages returns the actual page count for each PDF path using PyMuPDF.
// Returns (map[path]pages, error). A value of -1 means the PDF could not be opened.
func CountPDFPages(dataDir string, paths []string) (map[string]int, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	pythonCmd, ok := findPythonCommandFunc(dataDir)
	if !ok {
		return nil, fmt.Errorf("Python not available")
	}
	inputJSON, _ := json.Marshal(paths)
	cmd := exec.Command(pythonCmd, "-c", pythonCountPagesScript, string(inputJSON))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("count pages failed: %w: %s", err, stderr.String())
	}
	var result map[string]int
	payload, payloadErr := pythonJSONPayload(stdout.Bytes())
	if payloadErr != nil {
		return nil, fmt.Errorf("parse count-pages output: %w", payloadErr)
	}
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("parse count-pages output: %w", err)
	}
	return result, nil
}

func attSourceInfo(att domain.Attachment) (os.FileInfo, bool) {
	if att.Resolved && att.ResolvedPath != "" {
		info, err := os.Stat(att.ResolvedPath)
		if err == nil {
			return info, true
		}
	}
	return nil, false
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// It reads PDF path and output dir from sys.argv, writes JSON to stdout.
const pythonExtractFiguresScript = `
import json, sys, os, time, re
import fitz

if sys.platform == "win32":
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")

DEFAULTS = {
    "render_dpi": 200, "x_tolerance": 5, "y_tolerance": 5,
    "min_cluster_area_pt": 5000, "min_output_w_px": 150, "min_output_h_px": 120,
    "min_output_kb": 35, "min_pct_page": 8, "large_area_pct": 15,
    "max_text_ratio": 0.35,
    "anchor_detect_w": 20, "anchor_detect_h": 20,
    "anchor_fallback_w": 80, "anchor_fallback_h": 80,
    "anchor_expand_pt": 20, "max_anchor_page_pct": 0.90,
    # V4: aspect ratio bounds (scientific figures rarely exceed these)
    "min_aspect_ratio": 0.25, "max_aspect_ratio": 4.0,
    # V4: adaptive threshold trigger
    "adaptive_cand_limit": 40, "adaptive_pct_mult": 1.5, "adaptive_kb_mult": 1.4,
    # V6: cap candidate count after clustering, before text extraction/rendering.
    "max_candidates_per_page": 80, "max_union_page_pct": 0.85,
    "dense_fallback_min_clusters": 40, "dense_fallback_max_page_pct": 0.98,
    # V7: avoid pathological get_image_bbox/cluster scans on dense raster/vector pages.
    "max_image_bbox_scan": 50, "max_drawings_for_image_bbox": 20000,
    "max_drawings_for_cluster": 100000, "max_extreme_vector_images": 10,
    # V10: scanned/raster-only long documents should not pay get_drawings() per page.
    "raster_only_page_scan_pages": 50,
    # V11: large raster figures are usually page-sized screenshots; 150dpi is enough and faster.
    "large_raster_low_dpi_pct": 15, "raster_render_dpi": 150,
    # V12: quality filters for scanned books/theses and publisher/logo pages.
    "long_doc_pages": 50, "long_doc_min_raster_pct": 8,
    "edge_page_min_pct": 12, "edge_page_count": 2,
}

def get_image_anchor_sets(page, drawing_count=0, imgs=None):
    if imgs is None:
        imgs = page.get_images(full=True)
    if len(imgs) > DEFAULTS["max_image_bbox_scan"]:
        return [], [], len(imgs), True
    if drawing_count > DEFAULTS["max_drawings_for_image_bbox"] and len(imgs) > DEFAULTS["max_extreme_vector_images"]:
        return [], [], len(imgs), True
    anchors_detect, anchors_fallback = [], []
    seen_rects = set()
    ph = page.rect.height
    hl, fs = ph * 0.06, ph * (1 - 0.06)
    for img_info in imgs:
        xref = img_info[0]
        name = img_info[7]
        rects = []
        try:
            rects = page.get_image_rects(xref)
        except:
            rects = []
        if not rects:
            try:
                rects = [page.get_image_bbox(name)]
            except:
                rects = []
        for bbox in rects:
            if not bbox or bbox.width <= 0 or bbox.height <= 0:
                continue
            key = (round(bbox.x0, 1), round(bbox.y0, 1), round(bbox.x1, 1), round(bbox.y1, 1))
            if key in seen_rects:
                continue
            seen_rects.add(key)
            bw, bh, aspect = bbox.width, bbox.height, bbox.width / max(bbox.height, 1)
            cy = (bbox.y0 + bbox.y1) / 2
            if (cy < hl or cy > fs) and aspect > 17: continue
            rect = fitz.Rect(bbox)
            if not (bw < DEFAULTS["anchor_detect_w"] and bh < DEFAULTS["anchor_detect_h"]):
                anchors_detect.append(rect)
            if not (bw < DEFAULTS["anchor_fallback_w"] and bh < DEFAULTS["anchor_fallback_h"]):
                anchors_fallback.append(rect)
    return anchors_detect, anchors_fallback, len(imgs), False

def count_anchors_in_rect(rect, anchors):
    c = 0
    for a in anchors:
        center = fitz.Point((a.x0+a.x1)/2, (a.y0+a.y1)/2)
        if rect.contains(center) or rect.intersects(a):
            o = rect & a
            if o.width > 5 and o.height > 5: c += 1
    return c

def calc_text_density(page, rect):
    blocks = page.get_text("dict", clip=rect)["blocks"]
    ta, cc, parts = 0, 0, []
    for blk in blocks:
        if blk["type"] != 0: continue
        b = fitz.Rect(blk["bbox"])
        ta += b.width * b.height
        for line in blk.get("lines", []):
            for sp in line.get("spans", []):
                t = sp.get("text", "")
                cc += len(t); parts.append(t)
    tr = ta / max(rect.width * rect.height, 1)
    ft = "".join(parts).strip()
    norm = re.sub(r"\s+", " ", ft)
    cp = re.compile(r"^(?:F\s*I\s*G\s*U\s*R\s*E|图)\s+\.?\s*\d", re.I)
    return {"text_ratio": round(tr,3), "char_count": cc,
            "is_text_heavy": tr > DEFAULTS["max_text_ratio"],
            "is_caption": bool(cp.match(norm))}

def attach_caption(page, rect):
    cr = re.compile(r"^(?:F\s*I\s*G\s*U\s*R\s*E|图)\s+\.?\s*\d", re.I)
    margin, ext = 200, 120
    sr = fitz.Rect(rect.x0-margin, rect.y0, rect.x1+margin, rect.y1+ext) & page.rect
    best, bd = None, float("inf")
    found_inside = False
    for blk in page.get_text("dict", clip=sr)["blocks"]:
        if blk["type"] != 0: continue
        b = fitz.Rect(blk["bbox"])
        tp = "".join(sp.get("text","") for l in blk.get("lines",[]) for sp in l.get("spans",[]))
        nm = re.sub(r"\s+", " ", tp.strip())
        if not cr.match(nm): continue
        if rect.contains(b):
            found_inside = True
            continue
        below = b.y0 >= rect.y1-5 and b.y0 <= rect.y1+ext
        left = b.x1 <= rect.x0+5 and b.x1 >= rect.x0-ext
        right = b.x0 >= rect.x1-5 and b.x0 <= rect.x1+ext
        if not (below or left or right): continue
        d = (((b.x0+b.x1)/2-(rect.x0+rect.x1)/2)**2 + ((b.y0+b.y1)/2-(rect.y0+rect.y1)/2)**2)**0.5
        if d < bd: best, bd = b, d
    if best:
        nr = fitz.Rect(rect); nr |= best; return nr, True
    if found_inside:
        return rect, True
    return rect, False

def is_covered(anchor, clusters, gap=20):
    for c in clusters:
        e = fitz.Rect(c.x0-gap, c.y0-gap, c.x1+gap, c.y1+gap)
        if e.contains(fitz.Point((anchor.x0+anchor.x1)/2, (anchor.y0+anchor.y1)/2)): return True
    return False

# V4: confidence scoring 0-100
def calc_confidence(kb, anchors, has_caption, text_ratio, pct_page, aspect):
    s = 0
    if kb > 200: s += 25
    elif kb > 100: s += 18
    elif kb > 50: s += 10
    if anchors >= 3: s += 20
    elif anchors >= 1: s += 10
    if has_caption: s += 20
    if text_ratio < 0.10: s += 15
    elif text_ratio < 0.20: s += 8
    if 0.5 <= aspect <= 2.0: s += 12
    elif 0.3 <= aspect <= 3.0: s += 6
    if pct_page > 20: s += 8
    return min(s, 100)

# V4: cross-page size similarity check for header/footer dedup
def similar_size(a, b, tol=0.15):
    aw, ah = a.width*200/72, a.height*200/72
    bw, bh = b.width*200/72, b.height*200/72
    return abs(aw-bw)/max(aw,bw) < tol and abs(ah-bh)/max(ah,bh) < tol

def rank_candidate(c):
    rect, src = c
    score = rect.width * rect.height
    if src == "raster":
        score *= 1.2
    return score

def page_content_rect(page, margin_pct=0.06):
    r = page.rect
    mx, my = r.width * margin_pct, r.height * margin_pct
    return fitz.Rect(r.x0 + mx, r.y0 + my, r.x1 - mx, r.y1 - my) & r

def is_low_quality_raster_candidate(src, pp, pg, n_pages, aspect):
    if src != "raster":
        return False
    if n_pages >= DEFAULTS["long_doc_pages"] and pp < DEFAULTS["long_doc_min_raster_pct"]:
        return True
    if pg == 0 and pp < DEFAULTS["edge_page_min_pct"]:
        return True
    if pg >= max(n_pages - DEFAULTS["edge_page_count"], 1) and pp < DEFAULTS["edge_page_min_pct"] and (aspect > 2.5 or pp < 5):
        return True
    return False

def main():
    pdf_path, out_dir = sys.argv[1], sys.argv[2]
    os.makedirs(out_dir, exist_ok=True)
    doc = fitz.open(pdf_path)
    n_pages, figures, stats = len(doc), [], {}
    sk = {"clusters_total":0,"anchors_fallback":0,"kept":0,"filt_small":0,
          "filt_tiny":0,"filt_pct":0,"filt_no_anchor":0,"filt_text_heavy":0,
          "filt_caption":0,"filt_fullpage":0,"filt_covered":0,
          # V4 counters
          "filt_aspect":0,"filt_crosspage_dup":0,
          # V6 counter
          "candidate_cap":0,"dense_unions":0,"dense_page_fallback":0,
          "image_bbox_skipped_pages":0,"image_bbox_skipped_images":0,
          "cluster_skipped_pages":0,"cluster_skipped_drawings":0,
          "raster_only_pages":0,"filt_low_quality_raster":0}
    t0, fi = time.perf_counter(), 0

    # V3: cross-page dedup — shared across all pages to catch repeating headers/footers
    global_seen = []

    for pg in range(n_pages):
        p = doc[pg]
        pw, ph = p.rect.width*200/72, p.rect.height*200/72
        pa = p.rect.width*p.rect.height
        pre_imgs = p.get_images(full=True)
        raster_only_page = False
        if n_pages >= DEFAULTS["raster_only_page_scan_pages"] and pre_imgs:
            try:
                raster_only_page = len(p.get_text("blocks")) == 0
            except:
                raster_only_page = False
        if raster_only_page:
            dw = []
            sk["raster_only_pages"] += 1
        else:
            dw = p.get_drawings()
        extreme_vector_page = len(dw) > DEFAULTS["max_drawings_for_cluster"]
        an, fa, raw_images, image_bbox_skipped = get_image_anchor_sets(p, len(dw), pre_imgs)
        if image_bbox_skipped:
            sk["image_bbox_skipped_pages"] += 1
            sk["image_bbox_skipped_images"] += raw_images
        cl = []
        if dw and not extreme_vector_page:
            cl = p.cluster_drawings(drawings=dw, x_tolerance=DEFAULTS["x_tolerance"], y_tolerance=DEFAULTS["y_tolerance"])
        elif extreme_vector_page:
            sk["cluster_skipped_pages"] += 1
            sk["cluster_skipped_drawings"] += len(dw)
        sk["clusters_total"] += len(cl)

        # --- collect candidates ---
        cands = []
        cluster_rects = []
        for ci, r in enumerate(cl):
            if r.width*r.height >= DEFAULTS["min_cluster_area_pt"]:
                rr = fitz.Rect(r)
                cands.append((rr,"cluster"))
                cluster_rects.append(rr)
            else: sk["filt_small"] += 1
        raster_rects = []
        raster_candidates = []
        for a in fa:
            ap = (a.width*a.height)/pa
            if ap > DEFAULTS["max_anchor_page_pct"]: sk["filt_fullpage"]+=1; continue
            ex = fitz.Rect(max(0,a.x0-DEFAULTS["anchor_expand_pt"]), max(0,a.y0-DEFAULTS["anchor_expand_pt"]),
                       a.x1+DEFAULTS["anchor_expand_pt"], a.y1+DEFAULTS["anchor_expand_pt"]) & p.rect
            raster_rects.append(ex)
            if is_covered(a, cl): sk["filt_covered"]+=1; continue
            raster_candidates.append((ex,"raster"))

        if len(cluster_rects) > DEFAULTS["adaptive_cand_limit"]:
            ur = fitz.Rect(cluster_rects[0])
            for rr in cluster_rects[1:]:
                ur |= rr
            up = (ur.width * ur.height) / pa
            if up <= DEFAULTS["max_union_page_pct"] and ur.width * ur.height >= DEFAULTS["min_cluster_area_pt"]:
                cands.append((ur, "cluster"))
                sk["dense_unions"] += 1
        if len(raster_rects) > DEFAULTS["adaptive_cand_limit"]:
            ur = fitz.Rect(raster_rects[0])
            for rr in raster_rects[1:]:
                ur |= rr
            up = (ur.width * ur.height) / pa
            if up <= DEFAULTS["dense_fallback_max_page_pct"] and ur.width * ur.height >= DEFAULTS["min_cluster_area_pt"]:
                cands.append((ur, "raster"))
                sk["dense_unions"] += 1
                sk["anchors_fallback"] += 1
        else:
            cands.extend(raster_candidates)
            sk["anchors_fallback"] += len(raster_candidates)

        if len(cands) > DEFAULTS["max_candidates_per_page"]:
            cands.sort(key=rank_candidate, reverse=True)
            sk["candidate_cap"] += len(cands) - DEFAULTS["max_candidates_per_page"]
            cands = cands[:DEFAULTS["max_candidates_per_page"]]

        # V4: adaptive thresholding — tighten when too many candidates from this PDF
        adaptive = len(cands) > DEFAULTS["adaptive_cand_limit"]
        eff_pct = DEFAULTS["min_pct_page"] * (DEFAULTS["adaptive_pct_mult"] if adaptive else 1.0)
        eff_kb  = DEFAULTS["min_output_kb"] * (DEFAULTS["adaptive_kb_mult"] if adaptive else 1.0)

        seen, page_kept = [], 0
        for rc, src in cands:
            rw, rh = rc.width*200/72, rc.height*200/72
            if rw<DEFAULTS["min_output_w_px"] or rh<DEFAULTS["min_output_h_px"]:
                if src=="cluster": sk["filt_small"]+=1; continue

            # V4: aspect ratio filter (P0)
            ar = rw / max(rh, 1)
            if ar < DEFAULTS["min_aspect_ratio"] or ar > DEFAULTS["max_aspect_ratio"]:
                sk["filt_aspect"] += 1
                continue

            pp = (rw*rh)/(pw*ph)*100
            if is_low_quality_raster_candidate(src, pp, pg, n_pages, ar):
                sk["filt_low_quality_raster"] += 1
                continue
            if pp<eff_pct:
                if src=="cluster": sk["filt_pct"]+=1; continue
            na = count_anchors_in_rect(rc, an)
            if pp>=DEFAULTS["large_area_pct"] and na==0 and not image_bbox_skipped:
                sk["filt_no_anchor"]+=1
                continue
            td = calc_text_density(p, rc)
            if td["is_text_heavy"]: sk["filt_text_heavy"]+=1; continue
            if td["is_caption"] and na==0: sk["filt_caption"]+=1; continue

            # V3: same-page dedup (unchanged)
            if any((rc&s).width>50 and (rc&s).height>50 for s in seen): continue
            seen.append(rc)

            # V4: cross-page dedup (P3) — skip if similar size already kept on another page
            if any(similar_size(rc, gs) for gs in global_seen):
                sk["filt_crosspage_dup"] += 1
                continue

            rc2, hc = attach_caption(p, rc)
            render_dpi = DEFAULTS["render_dpi"]
            if src == "raster" and pp >= DEFAULTS["large_raster_low_dpi_pct"]:
                render_dpi = DEFAULTS["raster_render_dpi"]
            try:
                pix = p.get_pixmap(clip=fitz.Rect(rc2), dpi=render_dpi)
                ib = pix.tobytes("png"); kb = len(ib)/1024
            except: continue
            if kb<eff_kb: sk["filt_tiny"]+=1; continue

            # V4: confidence score (P1)
            conf = calc_confidence(kb, na, hc or td["is_caption"], td["text_ratio"], pp, ar)

            sk["kept"]+=1; page_kept+=1; fi+=1
            global_seen.append(rc)  # register for cross-page dedup
            fn=f"p{pg+1}_fig{fi}.png"
            with open(os.path.join(out_dir,fn),"wb") as f: f.write(ib)
            figures.append({"id":fi,"file":fn,"page":pg+1,"source":src,
                "size_px":f"{pix.width}x{pix.height}","size_pt":f"{rc2.width:.0f}x{rc2.height:.0f}",
                "kb":round(kb,1),"anchors":na,"has_caption":hc or td["is_caption"],
                "text_ratio":td["text_ratio"],"pct_page":round(pp,1),
                "confidence":conf})

        if page_kept == 0 and image_bbox_skipped:
            fr = page_content_rect(p)
            fpct = (fr.width * fr.height) / pa
            try:
                pix = p.get_pixmap(clip=fr, dpi=150)
                ib = pix.tobytes("png"); kb = len(ib)/1024
            except:
                pix = None; ib = None; kb = 0
            if ib and kb >= DEFAULTS["min_output_kb"]:
                fi += 1
                sk["kept"] += 1
                sk["dense_page_fallback"] += 1
                fn=f"p{pg+1}_fig{fi}.png"
                with open(os.path.join(out_dir,fn),"wb") as f: f.write(ib)
                figures.append({"id":fi,"file":fn,"page":pg+1,"source":"raster",
                    "size_px":f"{pix.width}x{pix.height}","size_pt":f"{fr.width:.0f}x{fr.height:.0f}",
                    "kb":round(kb,1),"anchors":0,"has_caption":False,
                    "text_ratio":0,"pct_page":round(fpct*100,1),
                    "confidence":20})
        elif page_kept == 0 and (len(cl) >= DEFAULTS["dense_fallback_min_clusters"] or len(raster_rects) >= DEFAULTS["dense_fallback_min_clusters"]):
            fallback_rects = raster_rects
            if not fallback_rects:
                fallback_rects = [fitz.Rect(r) for r in cl if r.width * r.height > 0]
            if fallback_rects:
                fr = fitz.Rect(fallback_rects[0])
                for rr in fallback_rects[1:]:
                    fr |= rr
                fr = fr & p.rect
                fpct = (fr.width * fr.height) / pa
                if fpct <= DEFAULTS["dense_fallback_max_page_pct"] and fr.width > 20 and fr.height > 20:
                    try:
                        pix = p.get_pixmap(clip=fr, dpi=150)
                        ib = pix.tobytes("png"); kb = len(ib)/1024
                    except:
                        pix = None; ib = None; kb = 0
                    if ib and kb >= DEFAULTS["min_output_kb"]:
                        fi += 1
                        sk["kept"] += 1
                        sk["dense_page_fallback"] += 1
                        fn=f"p{pg+1}_fig{fi}.png"
                        with open(os.path.join(out_dir,fn),"wb") as f: f.write(ib)
                        figures.append({"id":fi,"file":fn,"page":pg+1,"source":"cluster",
                            "size_px":f"{pix.width}x{pix.height}","size_pt":f"{fr.width:.0f}x{fr.height:.0f}",
                            "kb":round(kb,1),"anchors":count_anchors_in_rect(fr, an),"has_caption":False,
                            "text_ratio":0,"pct_page":round(fpct*100,1),
                            "confidence":25})
    doc.close()
    sk["elapsed_sec"]=round(time.perf_counter()-t0,2)
    sk["page_count"] = n_pages
    print(json.dumps({"figures":figures,"stats":sk,"error":None}, ensure_ascii=False, indent=2))

if __name__ == "__main__": main()
`

// ExtractFigures extracts figures from all PDF attachments of the given item.
// Output is organized as {outputDir}/{attachmentKey}/, matching fulltext cache layout.
func (r *LocalReader) ExtractFigures(ctx context.Context, item domain.Item, outputDir string) (ExtractFiguresResult, error) {
	result := ExtractFiguresResult{
		ItemKey: item.Key,
		Method:  "cluster_drawings_v13",
	}

	var pdfs []domain.Attachment
	for _, att := range item.Attachments {
		if strings.EqualFold(strings.TrimSpace(att.ContentType), "application/pdf") {
			pdfs = append(pdfs, att)
		}
	}
	if len(pdfs) == 0 {
		return result, fmt.Errorf("no PDF attachment found for item %s", item.Key)
	}

	absOutDir, err := filepath.Abs(outputDir)
	if err != nil {
		return result, err
	}

	pythonCmd, ok := findPythonCommandFunc(r.DataDir)
	if !ok {
		return result, fmt.Errorf("Python not available (data_dir: %s)", r.DataDir)
	}

	type pyFigure struct {
		ID         int     `json:"id"`
		File       string  `json:"file"`
		Page       int     `json:"page"`
		Source     string  `json:"source"`
		SizePx     string  `json:"size_px"`
		SizePt     string  `json:"size_pt"`
		KB         float64 `json:"kb"`
		Anchors    int     `json:"anchors"`
		Caption    string  `json:"caption,omitempty"`
		HasCaption bool    `json:"has_caption"`
		TextRatio  float64 `json:"text_ratio"`
		PctPage    float64 `json:"pct_page"`
		Confidence int     `json:"confidence"`
	}

	var totalElapsed float64
	var errs []string
	cache := newFigureCache(r.DataDir)
	cacheHits, cacheMisses := 0, 0
	seenPDFHashes := map[string]string{}

	for _, att := range pdfs {
		attKey := strings.TrimSpace(att.Key)
		attDir := filepath.Join(absOutDir, attKey)

		if !att.Resolved || att.ResolvedPath == "" {
			errs = append(errs, fmt.Sprintf("%s: PDF path not resolved", attKey))
			continue
		}
		if len(pdfs) > 1 {
			if hash, hashErr := fileSHA256(att.ResolvedPath); hashErr == nil {
				if _, duplicate := seenPDFHashes[hash]; duplicate {
					continue
				}
				seenPDFHashes[hash] = attKey
			}
		}

		// --- Cache hit: skip Python entirely ---
		if meta, ok := cache.Load(att); ok && cache.IsFresh(meta, att) && cachedFigureFilesAvailable(meta, attDir) {
			cacheHits++
			totalElapsed += meta.ElapsedSec
			result.PDFPath = att.ResolvedPath
			result.TotalPages += meta.TotalPages
			result.Figures = append(result.Figures, meta.Figures...)
			continue
		}
		cacheMisses++

		_ = os.RemoveAll(attDir)
		cmd := exec.CommandContext(ctx, pythonCmd, "-", att.ResolvedPath, attDir)
		cmd.Stdin = strings.NewReader(pythonExtractFiguresScript)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if runErr := cmd.Run(); runErr != nil {
			errMsg := strings.TrimSpace(stderr.String())
			if errMsg == "" {
				errMsg = runErr.Error()
			}
			_ = os.RemoveAll(attDir)
			errs = append(errs, fmt.Sprintf("%s: %s", attKey, errMsg))
			continue
		}

		var pyResult struct {
			Figures []pyFigure `json:"figures"`
			Stats   struct {
				ElapsedSec float64 `json:"elapsed_sec"`
				PageCount  int     `json:"page_count"`
			} `json:"stats"`
			Error *string `json:"error,omitempty"`
		}
		payload, payloadErr := pythonJSONPayload(stdout.Bytes())
		if payloadErr != nil {
			_ = os.RemoveAll(attDir)
			errs = append(errs, fmt.Sprintf("%s: parse error: %v", attKey, payloadErr))
			continue
		}
		if jsonErr := json.Unmarshal(payload, &pyResult); jsonErr != nil {
			_ = os.RemoveAll(attDir)
			errs = append(errs, fmt.Sprintf("%s: parse error: %v", attKey, jsonErr))
			continue
		}
		if pyResult.Error != nil && *pyResult.Error != "" {
			errs = append(errs, fmt.Sprintf("%s: %s", attKey, *pyResult.Error))
		}

		totalElapsed += pyResult.Stats.ElapsedSec
		result.TotalPages += pyResult.Stats.PageCount
		result.PDFPath = att.ResolvedPath // last processed PDF

		for _, f := range pyResult.Figures {
			f := f
			result.Figures = append(result.Figures, FigureInfo{
				ID:            f.ID,
				File:          f.File,
				Page:          f.Page,
				Source:        f.Source,
				SizePx:        f.SizePx,
				SizePt:        f.SizePt,
				KB:            f.KB,
				Anchors:       f.Anchors,
				Caption:       f.Caption,
				HasCaption:    f.HasCaption,
				TextRatio:     f.TextRatio,
				PctPage:       f.PctPage,
				Confidence:    f.Confidence,
				AttachmentKey: attKey,
			})
		}

		// Save successful result to cache
		attResult := ExtractFiguresResult{
			ItemKey:    attKey,
			PDFPath:    att.ResolvedPath,
			Method:     result.Method,
			ElapsedSec: pyResult.Stats.ElapsedSec,
			TotalPages: pyResult.Stats.PageCount,
		}
		offset := len(result.Figures) - len(pyResult.Figures)
		if offset >= 0 {
			attResult.Figures = result.Figures[offset:]
		}
		_ = cache.Save(att, attResult)
	}

	result.ElapsedSec = totalElapsed
	if len(errs) > 0 {
		result.Error = strings.Join(errs, "; ")
	}
	return result, nil
}
