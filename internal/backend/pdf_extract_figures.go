package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

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

// pythonExtractFiguresScript is the inline Python script passed to PyMuPDF.
// It reads PDF path and output dir from sys.argv, writes JSON to stdout.
const pythonExtractFiguresScript = `
import json, sys, os, time, re
import fitz

if sys.platform == "win32":
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")

DEFAULTS = {
    "render_dpi": 200, "x_tolerance": 5, "y_tolerance": 5,
    "min_cluster_area_pt": 5000, "min_output_w_px": 120, "min_output_h_px": 100,
    "min_output_kb": 15, "min_pct_page": 8, "large_area_pct": 15,
    "max_text_ratio": 0.35,
    "anchor_detect_w": 20, "anchor_detect_h": 20,
    "anchor_fallback_w": 80, "anchor_fallback_h": 80,
    "anchor_expand_pt": 20, "max_anchor_page_pct": 0.90,
}

def get_image_anchors(page, min_w=20, min_h=20):
    imgs = page.get_images(full=True)
    anchors = []
    ph = page.rect.height
    hl, fs = ph * 0.06, ph * (1 - 0.06)
    for img_info in imgs:
        name = img_info[7]
        try: bbox = page.get_image_bbox(name)
        except: continue
        bw, bh, aspect = bbox.width, bbox.height, bbox.width / max(bbox.height, 1)
        if bw < min_w and bh < min_h: continue
        cy = (bbox.y0 + bbox.y1) / 2
        if (cy < hl or cy > fs) and aspect > 17: continue
        anchors.append(fitz.Rect(bbox))
    return anchors

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
    norm = re.sub(r"\\s+", " ", ft)
    cp = re.compile(r"^F\\s*I\\s*G\\s*U\\s*R\\s*E\\s+\\.?\\s*\\d", re.I)
    return {"text_ratio": round(tr,3), "char_count": cc,
            "is_text_heavy": tr > DEFAULTS["max_text_ratio"],
            "is_caption": bool(cp.match(norm))}

def attach_caption(page, rect):
    cr = re.compile(r"^F\\s*I\\s*G\\s*U\\s*R\\s*E\\s+\\.?\\s*\\d", re.I)
    margin, ext = 200, 120
    sr = fitz.Rect(rect.x0-margin, rect.y0, rect.x1+margin, rect.y1+ext) & page.rect
    best, bd = None, float("inf")
    for blk in page.get_text("dict", clip=sr)["blocks"]:
        if blk["type"] != 0: continue
        b = fitz.Rect(blk["bbox"])
        if rect.contains(b): continue
        below = b.y0 >= rect.y1-5 and b.y0 <= rect.y1+ext
        left = b.x1 <= rect.x0+5 and b.x1 >= rect.x0-ext
        right = b.x0 >= rect.x1-5 and b.x0 <= rect.x1+ext
        if not (below or left or right): continue
        tp = "".join(sp.get("text","") for l in blk.get("lines",[]) for sp in l.get("spans",[]))
        nm = re.sub(r"\\s+", " ", tp.strip())
        if not cr.match(nm): continue
        d = (((b.x0+b.x1)/2-(rect.x0+rect.x1)/2)**2 + ((b.y0+b.y1)/2-(rect.y0+rect.y1)/2)**2)**0.5
        if d < bd: best, bd = b, d
    if best:
        nr = fitz.Rect(rect); nr |= best; return nr, True
    return rect, False

def is_covered(anchor, clusters, gap=20):
    for c in clusters:
        e = fitz.Rect(c.x0-gap, c.y0-gap, c.x1+gap, c.y1+gap)
        if e.contains(fitz.Point((anchor.x0+anchor.x1)/2, (anchor.y0+anchor.y1)/2)): return True
    return False

def main():
    pdf_path, out_dir = sys.argv[1], sys.argv[2]
    os.makedirs(out_dir, exist_ok=True)
    doc = fitz.open(pdf_path)
    n_pages, figures, stats = len(doc), [], {}
    sk = {"clusters_total":0,"anchors_fallback":0,"kept":0,"filt_small":0,
          "filt_tiny":0,"filt_pct":0,"filt_no_anchor":0,"filt_text_heavy":0,
          "filt_caption":0,"filt_fullpage":0,"filt_covered":0}
    t0, fi = time.perf_counter(), 0
    for pg in range(n_pages):
        p = doc[pg]
        pw, ph = p.rect.width*200/72, p.rect.height*200/72
        pa = p.rect.width*p.rect.height
        dw = p.get_drawings()
        an = get_image_anchors(p, DEFAULTS["anchor_detect_w"], DEFAULTS["anchor_detect_h"])
        fa = get_image_anchors(p, DEFAULTS["anchor_fallback_w"], DEFAULTS["anchor_fallback_h"])
        cl = []
        if dw: cl = p.cluster_drawings(drawings=dw, x_tolerance=DEFAULTS["x_tolerance"], y_tolerance=DEFAULTS["y_tolerance"])
        sk["clusters_total"] += len(cl)
        cands = []
        for ci, r in enumerate(cl):
            if r.width*r.height >= DEFAULTS["min_cluster_area_pt"]: cands.append((fitz.Rect(r),"cluster"))
            else: sk["filt_small"] += 1
        for a in fa:
            ap = (a.width*a.height)/pa
            if ap > DEFAULTS["max_anchor_page_pct"]: sk["filt_fullpage"]+=1; continue
            if is_covered(a, cl): sk["filt_covered"]+=1; continue
            ex = fitz.Rect(max(0,a.x0-DEFAULTS["anchor_expand_pt"]), max(0,a.y0-DEFAULTS["anchor_expand_pt"]),
                       a.x1+DEFAULTS["anchor_expand_pt"], a.y1+DEFAULTS["anchor_expand_pt"]) & p.rect
            cands.append((ex,"raster")); sk["anchors_fallback"]+=1
        seen, lfi = [], 0
        for rc, src in cands:
            rw, rh = rc.width*200/72, rc.height*200/72
            if rw<DEFAULTS["min_output_w_px"] or rh<DEFAULTS["min_output_h_px"]:
                if src=="cluster": sk["filt_small"]+=1; continue
            pp = (rw*rh)/(pw*ph)*100
            if pp<DEFAULTS["min_pct_page"]:
                if src=="cluster": sk["filt_pct"]+=1; continue
            na = count_anchors_in_rect(rc, an)
            if pp>=DEFAULTS["large_area_pct"] and na==0: sk["filt_no_anchor"]+=1; continue
            td = calc_text_density(p, rc)
            if td["is_text_heavy"]: sk["filt_text_heavy"]+=1; continue
            if td["is_caption"] and na==0: sk["filt_caption"]+=1; continue
            if any((rc&s).width>50 and (rc&s).height>50 for s in seen): continue
            seen.append(rc)
            rc2, hc = attach_caption(p, rc)
            try:
                pix = p.get_pixmap(clip=fitz.Rect(rc2), dpi=200)
                ib = pix.tobytes("png"); kb = len(ib)/1024
            except: continue
            if kb<DEFAULTS["min_output_kb"]: sk["filt_tiny"]+=1; continue
            sk["kept"]+=1; fi+=1
            fn=f"p{pg+1}_fig{fi}.png"
            with open(os.path.join(out_dir,fn),"wb") as f: f.write(ib)
            figures.append({"id":fi,"file":fn,"page":pg+1,"source":src,
                "size_px":f"{pix.width}x{pix.height}","size_pt":f"{rc2.width:.0f}x{rc2.height:.0f}",
                "kb":round(kb,1),"anchors":na,"has_caption":hc,
                "text_ratio":td["text_ratio"],"pct_page":round(pp,1)})
    doc.close()
    sk["elapsed_sec"]=round(time.perf_counter()-t0,2)
    print(json.dumps({"figures":figures,"stats":sk,"error":None}, ensure_ascii=False, indent=2))

if __name__ == "__main__": main()
`

// ExtractFigures extracts figures from all PDF attachments of the given item.
// Output is organized as {outputDir}/{attachmentKey}/, matching fulltext cache layout.
func (r *LocalReader) ExtractFigures(ctx context.Context, item domain.Item, outputDir string) (ExtractFiguresResult, error) {
	result := ExtractFiguresResult{
		ItemKey: item.Key,
		Method:  "cluster_drawings_v5b",
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
	}

	var totalElapsed float64
	var errs []string

	for _, att := range pdfs {
		attKey := strings.TrimSpace(att.Key)
		attDir := filepath.Join(absOutDir, attKey)

		if !att.Resolved || att.ResolvedPath == "" {
			errs = append(errs, fmt.Sprintf("%s: PDF path not resolved", attKey))
			continue
		}

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
			errs = append(errs, fmt.Sprintf("%s: %s", attKey, errMsg))
			continue
		}

		var pyResult struct {
			Figures []pyFigure `json:"figures"`
			Stats   struct {
				ElapsedSec float64 `json:"elapsed_sec"`
			} `json:"stats"`
			Error *string `json:"error,omitempty"`
		}
		if jsonErr := json.Unmarshal(stdout.Bytes(), &pyResult); jsonErr != nil {
			errs = append(errs, fmt.Sprintf("%s: parse error: %v", attKey, jsonErr))
			continue
		}
		if pyResult.Error != nil && *pyResult.Error != "" {
			errs = append(errs, fmt.Sprintf("%s: %s", attKey, *pyResult.Error))
		}

		totalElapsed += pyResult.Stats.ElapsedSec
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
				AttachmentKey: attKey,
			})
		}
	}

	result.ElapsedSec = totalElapsed
	if len(errs) > 0 {
		result.Error = strings.Join(errs, "; ")
	}
	return result, nil
}
