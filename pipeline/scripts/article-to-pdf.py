#!/usr/bin/env python3
"""Convert markdown article to styled PDF using reportlab.

Usage: python3 article-to-pdf.py <input.md> [output.pdf]

If output.pdf is omitted, saves to ~/Documents/AgentForge/<filename>.pdf

Image handling:
  - ![alt](path) references are resolved relative to the markdown file's directory
  - If not found there, also checks relative to the workspace root (~/workspace/agentforge)
  - Images are embedded at max 160mm width, preserving aspect ratio
"""
import re, os, sys
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.colors import HexColor
from reportlab.lib.units import mm
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle, Preformatted, HRFlowable, Image as RLImage

DARK = HexColor("#1a1a1a")
MEDIUM = HexColor("#333333")
LIGHT = HexColor("#666666")
BLUE = HexColor("#3b82f6")
BG = HexColor("#f5f5f5")
BORDER = HexColor("#cccccc")

WORKSPACE = os.path.expanduser("~/workspace/agentforge")


def resolve_image_path(img_ref, md_dir):
    """Resolve an image reference to an absolute path.

    Checks in order:
    1. Relative to the markdown file's directory
    2. Relative to the workspace root
    3. As an absolute path
    """
    # Strip any query params or fragments
    img_path = img_ref.split("?")[0].split("#")[0]

    # 1. Relative to markdown file
    candidate = os.path.normpath(os.path.join(md_dir, img_path))
    if os.path.exists(candidate):
        return candidate

    # 2. Relative to workspace
    candidate = os.path.normpath(os.path.join(WORKSPACE, img_path))
    if os.path.exists(candidate):
        return candidate

    # 3. As-is (absolute or CWD-relative)
    if os.path.exists(img_path):
        return img_path

    return None


def build(inp, out):
    md_dir = os.path.dirname(os.path.abspath(inp))

    with open(inp) as f:
        content = f.read()

    # Strip YAML frontmatter
    content = re.sub(r"^---\n.*?\n---\n", "", content, flags=re.DOTALL)

    S = getSampleStyleSheet()

    def PS(n, **kw):
        return ParagraphStyle(n, parent=S["Normal"], **kw)

    ts = PS("T", fontSize=22, leading=28, textColor=DARK, fontName="Helvetica-Bold")
    h2 = PS("H2", fontSize=16, leading=22, textColor=DARK, spaceBefore=16, fontName="Helvetica-Bold")
    h3 = PS("H3", fontSize=13, leading=18, textColor=MEDIUM, spaceBefore=12, fontName="Helvetica-Bold")
    bs = PS("B", fontSize=11, leading=17, textColor=MEDIUM, fontName="Helvetica")
    cs = PS("C", fontSize=9, leading=14, textColor=LIGHT, fontName="Courier", backColor=BG, borderPad=8, leftIndent=10)
    bbs = PS("BB", fontSize=11, leading=17, textColor=MEDIUM, fontName="Helvetica-Bold")

    def fmt(t):
        return re.sub(r"\*\*(.*?)\*\*", r"<b>\1</b>", t)

    story = [Spacer(1, 10 * mm)]
    lines = content.split("\n")
    i = 0
    in_code = False
    code_buf = []
    tbl_buf = []
    missing_images = []

    while i < len(lines):
        ln = lines[i]

        # Code blocks
        if ln.startswith("```"):
            if in_code:
                story.append(Preformatted("\n".join(code_buf), cs))
                story.append(Spacer(1, 4 * mm))
                code_buf = []
                in_code = False
            else:
                in_code = True
            i += 1
            continue

        if in_code:
            code_buf.append(ln)
            i += 1
            continue

        # Image references: ![alt](path)
        img_match = re.match(r"!\[([^\]]*)\]\(([^)]+)\)", ln.strip())
        if img_match:
            alt_text = img_match.group(1)
            img_ref = img_match.group(2)
            resolved = resolve_image_path(img_ref, md_dir)
            if resolved:
                try:
                    img = RLImage(resolved)
                    # Scale to max 160mm width, preserving aspect ratio
                    max_w = 160 * mm
                    max_h = 120 * mm
                    img_w, img_h = img.imageWidth, img.imageHeight
                    scale = min(max_w / img_w, max_h / img_h)
                    img.drawWidth = img_w * scale
                    img.drawHeight = img_h * scale
                    story.append(Spacer(1, 4 * mm))
                    story.append(img)
                    if alt_text:
                        story.append(Paragraph(
                            f"<i>{alt_text}</i>",
                            PS("IC", fontSize=9, textColor=LIGHT, fontName="Helvetica-Oblique")
                        ))
                    story.append(Spacer(1, 6 * mm))
                except Exception as e:
                    missing_images.append(f"  Could not embed {img_ref}: {e}")
                    story.append(Paragraph(
                        f"[Image: {alt_text or img_ref}]",
                        PS("MI", fontSize=9, textColor=LIGHT, fontName="Courier")
                    ))
            else:
                missing_images.append(f"  Image not found: {img_ref} (tried relative to {md_dir} and {WORKSPACE})")
                story.append(Paragraph(
                    f"[Image: {alt_text or img_ref}]",
                    PS("MI", fontSize=9, textColor=LIGHT, fontName="Courier")
                ))
            i += 1
            continue

        # Tables
        if ln.startswith("|") and not ln.startswith("|---"):
            tbl_buf.append([x.strip() for x in ln.strip().strip("|").split("|")])
            i += 1
            if i >= len(lines) or not lines[i].startswith("|"):
                if len(tbl_buf) > 1:
                    nc = len(tbl_buf[0])
                    cw = [(170 * mm / nc)] * nc
                    td = []
                    for ri, row in enumerate(tbl_buf):
                        wr = []
                        for cell in row:
                            ps = ParagraphStyle(
                                "TH" if ri == 0 else "TD",
                                fontName="Helvetica-Bold" if ri == 0 else "Helvetica",
                                fontSize=9,
                                textColor=DARK if ri == 0 else MEDIUM,
                            )
                            wr.append(Paragraph(fmt(cell), ps))
                        td.append(wr)
                    t = Table(td, colWidths=cw, repeatRows=1)
                    t.setStyle(TableStyle([
                        ("BACKGROUND", (0, 0), (-1, 0), HexColor("#e8e8e8")),
                        ("FONTNAME", (0, 0), (-1, 0), "Helvetica-Bold"),
                        ("FONTNAME", (0, 1), (-1, -1), "Helvetica"),
                        ("FONTSIZE", (0, 0), (-1, -1), 9),
                        ("GRID", (0, 0), (-1, -1), 0.5, BORDER),
                        ("ROWBACKGROUNDS", (0, 1), (-1, -1), [HexColor("#ffffff"), HexColor("#fafafa")]),
                        ("TOPPADDING", (0, 0), (-1, -1), 7),
                        ("BOTTOMPADDING", (0, 0), (-1, -1), 7),
                        ("LEFTPADDING", (0, 0), (-1, -1), 10),
                    ]))
                    story.append(Spacer(1, 4 * mm))
                    story.append(t)
                    story.append(Spacer(1, 4 * mm))
                tbl_buf = []
            continue

        if ln.startswith("|---"):
            i += 1
            continue

        # Empty lines
        if not ln.strip():
            story.append(Spacer(1, 2 * mm))
            i += 1
            continue

        # H1
        if ln.startswith("# ") and not ln.startswith("## "):
            story.append(Paragraph(fmt(ln[2:].strip()), ts))
            story.append(HRFlowable(width="100%", thickness=3, color=BLUE, spaceAfter=6 * mm))
            i += 1
            continue

        # H2
        if ln.startswith("## "):
            story.append(Paragraph(fmt(ln[3:].strip()), h2))
            story.append(HRFlowable(width="100%", thickness=0.5, color=BORDER, spaceAfter=3 * mm))
            i += 1
            continue

        # H3
        if ln.startswith("### "):
            story.append(Paragraph(fmt(ln[4:].strip()), h3))
            i += 1
            continue

        # Bold-only line
        if ln.startswith("**") and ln.endswith("**"):
            story.append(Paragraph(fmt(ln.strip("*")), bbs))
            i += 1
            continue

        # Normal paragraph
        story.append(Paragraph(fmt(ln), bs))
        i += 1

    def footer(canvas, doc):
        canvas.saveState()
        canvas.setFont("Helvetica", 8)
        canvas.setFillColor(LIGHT)
        canvas.drawString(20 * mm, A4[1] - 15 * mm, "AutoRanker Content Pipeline")
        # Extract date from frontmatter or use file mtime
        date_str = "2026-05-10"
        canvas.drawRightString(A4[0] - 20 * mm, A4[1] - 15 * mm, date_str)
        canvas.drawCentredString(A4[0] / 2, 12 * mm, "Page " + str(canvas.getPageNumber()))
        canvas.restoreState()

    doc = SimpleDocTemplate(
        out, pagesize=A4,
        topMargin=25 * mm, bottomMargin=20 * mm,
        leftMargin=20 * mm, rightMargin=20 * mm,
    )
    doc.build(story, onFirstPage=footer, onLaterPages=footer)

    size = os.path.getsize(out)
    print(f"PDF: {out} ({size} bytes)")
    if missing_images:
        print("WARN: Missing images:")
        for m in missing_images:
            print(m)


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <input.md> [output.pdf]")
        sys.exit(1)
    inp = sys.argv[1]
    out = sys.argv[2] if len(sys.argv) >= 3 else os.path.expanduser(
        "~/Documents/AgentForge/" + os.path.splitext(os.path.basename(inp))[0] + ".pdf"
    )
    os.makedirs(os.path.dirname(out), exist_ok=True)
    build(inp, out)
