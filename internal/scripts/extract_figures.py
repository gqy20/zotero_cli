"""
extract_figures.py — PDF Figure 提取（v5b: cluster_drawings + 位图锚点回退）

通过 stdin 接收内联调用，JSON 输出到 stdout。
用法: python - <script> <pdf_path> <output_dir>

双路径策略:
  Path A (矢量): cluster_drawings() → 聚类矢量图形为 Figure
  Path B (位图):   大尺寸图片锚点 → 未被 Path A 覆盖的独立大图

过滤链:
  1. 面积/尺寸过滤
  2. 锚点检测（大面积无锚点 = Abstract 等非 Figure 区域）
  3. 文字密度检测（文字占比 >35% = 纯文本区）
  4. Caption 模式检测（无锚点的 caption 块）
  5. 全页扫描跳过

增强:
  - Caption 吸附（搜索周围 "FIGURE N" 文本并扩展包含）
  - 宽松正则匹配 PDF 字间距排版（"FI G U R E"）
"""

import sys
import os
import json
import re
import time
import fitz

if sys.platform == "win32":
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")

# ============================================================
# 配置参数（可通过 JSON 覆盖）
# ============================================================
DEFAULTS = {
    "render_dpi": 200,
    "x_tolerance": 5,
    "y_tolerance": 5,
    "min_cluster_area_pt": 5000,
    "min_output_w_px": 120,
    "min_output_h_px": 100,
    "min_output_kb": 15,
    "min_pct_page": 8,
    "large_area_pct": 15,
    "max_text_ratio": 0.35,
    "anchor_detect_w": 20,
    "anchor_detect_h": 20,
    "anchor_fallback_w": 80,
    "anchor_fallback_h": 80,
    "anchor_expand_pt": 20,
    "max_anchor_page_pct": 0.90,
}


def get_image_anchors(page, min_w=20, min_h=20):
    """获取有意义的图片锚点。"""
    imgs = page.get_images(full=True)
    anchors = []
    ph = page.rect.height
    header_limit = ph * 0.06
    footer_start = ph * (1 - 0.06)
    for img_info in imgs:
        name = img_info[7]
        try:
            bbox = page.get_image_bbox(name)
        except Exception:
            continue
        bw, bh = bbox.width, bbox.height
        aspect = bw / max(bh, 1)
        if bw < min_w and bh < min_h:
            continue
        cy = (bbox.y0 + bbox.y1) / 2
        if (cy < header_limit or cy > footer_start) and aspect > 17:
            continue
        anchors.append(fitz.Rect(bbox))
    return anchors


def count_anchors_in_rect(rect, anchors):
    count = 0
    for a in anchors:
        center = fitz.Point((a.x0 + a.x1) / 2, (a.y0 + a.y1) / 2)
        if rect.contains(center) or rect.intersects(a):
            overlap = rect & a
            if overlap.width > 5 and overlap.height > 5:
                count += 1
    return count


def calc_text_density(page, rect):
    """计算区域文字密度 + caption 检测。"""
    blocks = page.get_text("dict", clip=rect)["blocks"]
    total_text_area = 0
    char_count = 0
    raw_text_parts = []
    for blk in blocks:
        if blk["type"] != 0:
            continue
        bbox = fitz.Rect(blk["bbox"])
        total_text_area += (bbox.width * bbox.height)
        for line in blk.get("lines", []):
            for span in line.get("spans", []):
                t = span.get("text", "")
                char_count += len(t)
                raw_text_parts.append(t)
    area = rect.width * rect.height
    text_ratio = total_text_area / max(area, 1)
    full_text = "".join(raw_text_parts).strip()
    normalized = re.sub(r"\s+", " ", full_text)
    caption_pattern = re.compile(
        r"^F\s*I\s*G\s*U\s*R\s*E\s+\.?\s*\d", re.IGNORECASE)
    return {
        "text_ratio": round(text_ratio, 3),
        "char_count": char_count,
        "is_text_heavy": text_ratio > DEFAULTS["max_text_ratio"],
        "is_caption": bool(caption_pattern.match(normalized)),
        "caption_preview": normalized[:120] if bool(caption_pattern.match(normalized)) else "",
    }


def attach_caption(page, rect):
    """检测并吸附 Figure caption。"""
    caption_re = re.compile(
        r"^F\s*I\s*G\s*U\s*R\s*E\s+\.?\s*\d", re.IGNORECASE)
    margin, ext = 200, 120
    search_rect = fitz.Rect(
        rect.x0 - margin, rect.y0,
        rect.x1 + margin, rect.y1 + ext,
    ) & page.rect
    blocks = page.get_text("dict", clip=search_rect)["blocks"]
    best_caption = None
    best_dist = float("inf")
    for blk in blocks:
        if blk["type"] != 0:
            continue
        bbox = fitz.Rect(blk["bbox"])
        if rect.contains(bbox):
            continue
        below = bbox.y0 >= rect.y1 - 5 and bbox.y0 <= rect.y1 + ext
        left_side = bbox.x1 <= rect.x0 + 5 and bbox.x1 >= rect.x0 - ext
        right_side = bbox.x0 >= rect.x1 - 5 and bbox.x0 <= rect.x1 + ext
        if not (below or left_side or right_side):
            continue
        text_parts = []
        for line in blk.get("lines", []):
            for span in line.get("spans", []):
                text_parts.append(span.get("text", ""))
        full_text = "".join(text_parts).strip()
        normalized = re.sub(r"\s+", " ", full_text)
        if not caption_re.match(normalized):
            continue
        cx = (bbox.x0 + bbox.x1) / 2
        cy = (bbox.y0 + bbox.y1) / 2
        rx = (rect.x0 + rect.x1) / 2
        ry = (rect.y0 + rect.y1) / 2
        dist = ((cx - rx) ** 2 + (cy - ry) ** 2) ** 0.5
        if dist < best_dist:
            best_caption = bbox
            best_dist = dist
    if best_caption:
        new_rect = fitz.Rect(rect)
        new_rect |= best_caption
        return new_rect, True
    return rect, False


def is_covered_by_clusters(anchor, clusters, gap=20):
    for c in clusters:
        expanded = fitz.Rect(c.x0 - gap, c.y0 - gap, c.x1 + gap, c.y1 + gap)
        if expanded.contains(fitz.Point((anchor.x0 + anchor.x1) / 2,
                                       (anchor.y0 + anchor.y1) / 2)):
            return True
    return False


def extract_figures(pdf_path, output_dir, overrides=None):
    """
    从单个 PDF 提取所有 Figure。

    返回 dict: { figures: [...], stats: {...], error: None|str }
    """
    cfg = dict(DEFAULTS)
    if overrides:
        cfg.update(overrides)

    output_dir = os.path.abspath(output_dir)
    os.makedirs(output_dir, exist_ok=True)

    try:
        doc = fitz.open(pdf_path)
    except Exception as e:
        return {"figures": [], "stats": {}, "error": str(e)}

    n_pages = len(doc)
    all_figures = []
    stats = {
        "clusters_total": 0, "anchors_fallback": 0, "kept": 0,
        "filt_small": 0, "filt_tiny": 0, "filt_pct": 0,
        "filt_no_anchor": 0, "filt_text_heavy": 0, "filt_caption": 0,
        "filt_fullpage": 0, "filt_covered": 0,
    }
    t0 = time.perf_counter()
    fig_idx = 0

    for pg_num in range(n_pages):
        page = doc[pg_num]
        pw_px = page.rect.width * cfg["render_dpi"] / 72
        ph_px = page.rect.height * cfg["render_dpi"] / 72
        page_area = page.rect.width * page.rect.height

        drawings = page.get_drawings()
        anchors = get_image_anchors(page, cfg["anchor_detect_w"], cfg["anchor_detect_h"])
        fallback_anchors = get_image_anchors(page, cfg["anchor_fallback_w"], cfg["anchor_fallback_h"])

        # Path A: 矢量聚类
        clusters = []
        if drawings:
            clusters = page.cluster_drawings(
                drawings=drawings,
                x_tolerance=cfg["x_tolerance"],
                y_tolerance=cfg["y_tolerance"],
            )
        stats["clusters_total"] += len(clusters)

        # 收集候选区域
        candidates = []

        # A1: cluster 候选
        for ci, rect in enumerate(clusters):
            area_pt = rect.width * rect.height
            if area_pt < cfg["min_cluster_area_pt"]:
                stats["filt_small"] += 1
                continue
            candidates.append((fitz.Rect(rect), "cluster"))

        # A2: 位图回退
        for anchor in fallback_anchors:
            anchor_pct = (anchor.width * anchor.height) / page_area
            if anchor_pct > cfg["max_anchor_page_pct"]:
                stats["filt_fullpage"] += 1
                continue
            if is_covered_by_clusters(anchor, clusters):
                stats["filt_covered"] += 1
                continue
            expanded = fitz.Rect(
                max(0, anchor.x0 - cfg["anchor_expand_pt"]),
                max(0, anchor.y0 - cfg["anchor_expand_pt"]),
                anchor.x1 + cfg["anchor_expand_pt"],
                anchor.y1 + cfg["anchor_expand_pt"],
            ) & page.rect
            candidates.append((expanded, "raster"))
            stats["anchors_fallback"] += 1

        if not candidates:
            continue

        # 统一过滤 + 渲染
        seen_rects = []

        for rect, source in candidates:
            rw_px = rect.width * cfg["render_dpi"] / 72
            rh_px = rect.height * cfg["render_dpi"] / 72
            if rw_px < cfg["min_output_w_px"] or rh_px < cfg["min_output_h_px"]:
                if source == "cluster":
                    stats["filt_small"] += 1
                continue

            pct_page = (rw_px * rh_px) / (pw_px * ph_px) * 100
            if pct_page < cfg["min_pct_page"]:
                if source == "cluster":
                    stats["filt_pct"] += 1
                continue

            n_anchors = count_anchors_in_rect(rect, anchors)
            is_large = pct_page >= cfg["large_area_pct"]
            if is_large and n_anchors == 0:
                stats["filt_no_anchor"] += 1
                continue

            td = calc_text_density(page, rect)
            if td["is_text_heavy"]:
                stats["filt_text_heavy"] += 1
                continue
            if td["is_caption"] and n_anchors == 0:
                stats["filt_caption"] += 1
                continue

            # 去重
            dup = any((rect & sr).width > 50 and (rect & sr).height > 50
                     for sr in seen_rects)
            if dup:
                continue
            seen_rects.append(rect)

            # Caption 吸附
            rect, has_caption = attach_caption(page, rect)

            # 渲染
            clip = fitz.Rect(rect)
            try:
                pix = page.get_pixmap(clip=clip, dpi=cfg["render_dpi"])
                img_bytes = pix.tobytes("png")
                img_kb = len(img_bytes) / 1024
            except Exception:
                continue

            if img_kb < cfg["min_output_kb"]:
                stats["filt_tiny"] += 1
                continue

            # 保存
            stats["kept"] += 1
            fig_idx += 1
            fname = f"p{pg_num + 1}_fig{fig_idx}.png"
            fpath = os.path.join(output_dir, fname)
            with open(fpath, "wb") as f:
                f.write(img_bytes)

            all_figures.append({
                "id": fig_idx,
                "file": fname,
                "page": pg_num + 1,
                "source": source,
                "size_px": f"{pix.width}x{pix.height}",
                "size_pt": f"{rect.width:.0f}x{rect.height:.0f}",
                "kb": round(img_kb, 1),
                "anchors": n_anchors,
                "has_caption": has_caption,
                "text_ratio": td["text_ratio"],
                "pct_page": round(pct_page, 1),
            })

    elapsed = time.perf_counter() - t0
    doc.close()

    stats["elapsed_sec"] = round(elapsed, 2)
    return {
        "figures": all_figures,
        "stats": stats,
        "error": None,
    }


if __name__ == "__main__":
    # 直接运行模式（用于测试/调试）
    if len(sys.argv) >= 3:
        pdf_path = sys.argv[1]
        output_dir = sys.argv[2]
        result = extract_figures(pdf_path, output_dir)
        print(json.dumps(result, ensure_ascii=False, indent=2))
    else:
        print(json.dumps({
            "error": f"Usage: python {sys.argv[0]} <pdf_path> <output_dir>",
            "figures": [],
            "stats": {},
        }, ensure_ascii=False, indent=2))
