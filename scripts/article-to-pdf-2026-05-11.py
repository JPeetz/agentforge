#!/usr/bin/env python3
"""Convert the May 11 article markdown to a PDF with the hero image embedded."""

import os
import re
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import mm
from reportlab.lib.colors import HexColor
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, Image as RLImage,
    Table, TableStyle, HRFlowable
)
from reportlab.lib.enums import TA_CENTER

WORKSPACE = os.path.expanduser("~/workspace/agentforge")
ARTICLE_MD = os.path.join(WORKSPACE, "pipeline/drafts/2026-05-11-ollama-vs-cloud-api.md")
HERO_IMG = os.path.join(WORKSPACE, "pipeline/images/2026-05-11-ollama-vs-cloud-api-hero.png")
OUTPUT_PDF = os.path.expanduser("~/Documents/AgentForge/2026-05-11-ollama-vs-cloud-api.pdf")

BRAND_BLUE = HexColor("#1a73e8")
BRAND_DARK = HexColor("#1a1a2e")
BRAND_GRAY = HexColor("#5f6368")
BRAND_LIGHT = HexColor("#f8f9fa")

def build_pdf():
    doc = SimpleDocTemplate(
        OUTPUT_PDF, pagesize=A4,
        leftMargin=20*mm, rightMargin=20*mm,
        topMargin=20*mm, bottomMargin=20*mm,
    )

    styles = getSampleStyleSheet()
    styles.add(ParagraphStyle("BrandTitle", parent=styles["Title"], fontSize=24, leading=30, textColor=BRAND_DARK, spaceAfter=6, fontName="Helvetica-Bold"))
    styles.add(ParagraphStyle("BrandH1", parent=styles["Heading1"], fontSize=18, leading=24, textColor=BRAND_DARK, spaceBefore=16, spaceAfter=8, fontName="Helvetica-Bold"))
    styles.add(ParagraphStyle("BrandH2", parent=styles["Heading2"], fontSize=14, leading=20, textColor=BRAND_BLUE, spaceBefore=12, spaceAfter=6, fontName="Helvetica-Bold"))
    styles.add(ParagraphStyle("BrandBody", parent=styles["Normal"], fontSize=11, leading=16, textColor=BRAND_DARK, spaceAfter=6, fontName="Helvetica"))
    styles.add(ParagraphStyle("BrandCode", parent=styles["Code"], fontSize=9, leading=13, textColor=BRAND_DARK, backColor=BRAND_LIGHT, spaceAfter=8, fontName="Courier", leftIndent=10, rightIndent=10, borderPad=4))
    styles.add(ParagraphStyle("Footer", parent=styles["Normal"], fontSize=8, textColor=BRAND_GRAY, alignment=TA_CENTER))

    story = []

    with open(ARTICLE_MD, "r") as f:
        md_text = f.read()

    md_text = re.sub(r"^---\n.*?\n---\n", "", md_text, flags=re.DOTALL)
    lines = md_text.split("\n")
    i = 0
    in_code = False
    code_lines = []
    table_rows = []

    def flush_table():
        if table_rows:
            clean_rows = []
            for row in table_rows:
                cells = [c.strip() for c in row.split("|")]
                cells = [c for c in cells if c]
                if cells and not all(re.match(r"^[-:]+$", c) for c in cells):
                    clean_rows.append(cells)
            if clean_rows:
                t = Table(clean_rows, repeatRows=1)
                t.setStyle(TableStyle([
                    ("BACKGROUND", (0, 0), (-1, 0), BRAND_BLUE),
                    ("TEXTCOLOR", (0, 0), (-1, 0), HexColor("#ffffff")),
                    ("FONTNAME", (0, 0), (-1, 0), "Helvetica-Bold"),
                    ("FONTSIZE", (0, 0), (-1, -1), 9),
                    ("ALIGN", (0, 0), (-1, -1), "LEFT"),
                    ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
                    ("GRID", (0, 0), (-1, -1), 0.5, BRAND_GRAY),
                    ("ROWBACKGROUNDS", (0, 1), (-1, -1), [HexColor("#ffffff"), BRAND_LIGHT]),
                    ("TOPPADDING", (0, 0), (-1, -1), 6),
                    ("BOTTOMPADDING", (0, 0), (-1, -1), 6),
                    ("LEFTPADDING", (0, 0), (-1, -1), 8),
                    ("RIGHTPADDING", (0, 0), (-1, -1), 8),
                ]))
                story.append(t)
                story.append(Spacer(1, 8*mm))

    while i < len(lines):
        line = lines[i]

        img_match = re.match(r"!\[([^\]]*)\]\(([^)]+)\)", line)
        if img_match:
            img_path = img_match.group(2)
            if not os.path.isabs(img_path):
                img_path = os.path.join(os.path.dirname(ARTICLE_MD), img_path)
            if os.path.exists(img_path):
                img = RLImage(img_path, width=160*mm, height=90*mm)
                story.append(img)
                story.append(Spacer(1, 4*mm))
            else:
                print(f"WARNING: Image not found: {img_path}")
            i += 1
            continue

        if line.startswith("```"):
            if in_code:
                code_text = "\n".join(code_lines)
                story.append(Paragraph(code_text.replace(" ", "&nbsp;").replace("<", "&lt;").replace(">", "&gt;"), styles["BrandCode"]))
                code_lines = []
                in_code = False
            else:
                in_code = True
            i += 1
            continue

        if in_code:
            code_lines.append(line)
            i += 1
            continue

        if line.startswith("|"):
            table_rows.append(line)
            i += 1
            continue
        else:
            flush_table()
            table_rows = []

        if line.startswith("# "):
            flush_table()
            story.append(Paragraph(line[2:].strip(), styles["BrandTitle"]))
            story.append(HRFlowable(width="100%", thickness=2, color=BRAND_BLUE, spaceAfter=8*mm))
        elif line.startswith("## "):
            flush_table()
            story.append(Paragraph(line[3:].strip(), styles["BrandH1"]))
        elif line.startswith("### "):
            flush_table()
            story.append(Paragraph(line[4:].strip(), styles["BrandH2"]))
        elif line.strip() == "":
            story.append(Spacer(1, 3*mm))
        else:
            text = re.sub(r"\*\*(.+?)\*\*", r"<b>\1</b>", line)
            text = re.sub(r"`(.+?)`", r'<font name="Courier" color="#1a73e8">\1</font>', text)
            story.append(Paragraph(text, styles["BrandBody"]))

        i += 1

    flush_table()
    story.append(Spacer(1, 10*mm))
    story.append(HRFlowable(width="100%", thickness=0.5, color=BRAND_GRAY))
    story.append(Paragraph("AutoRanker &mdash; Ollama vs Cloud API &mdash; 2026-05-11", styles["Footer"]))

    doc.build(story)
    size_kb = os.path.getsize(OUTPUT_PDF) / 1024
    print(f"PDF generated: {OUTPUT_PDF} ({size_kb:.1f} KB)")

if __name__ == "__main__":
    build_pdf()
