#!/usr/bin/env python3
"""Generate the Promptsheon social-preview.png (1280x640).

This is a build helper — invoked once to materialise the PNG
that GitHub uses for the repository's social-share card. The
SVG at assets/social-preview.svg is the source of truth; the
PNG is a rasterised derivative for platforms that can't render
SVG (LinkedIn, Slack, the GitHub social-preview uploader).
"""
from __future__ import annotations

from PIL import Image, ImageDraw, ImageFont
from pathlib import Path

OUT = Path(__file__).resolve().parents[1] / "assets" / "social-preview.png"
WIDTH, HEIGHT = 1280, 640

BG = (15, 17, 23)
MARK_BG_TOP = (25, 27, 38)
MARK_BG_BOT = (11, 13, 20)
MARK_FILL_TOP = (164, 181, 214)
MARK_FILL_MID = (108, 117, 145)
MARK_FILL_BOT = (30, 34, 51)
WORDMARK_TOP = (255, 255, 255)
WORDMARK_BOT = (199, 207, 227)
TAGLINE = (157, 166, 189)
SUBTAG = (108, 117, 145)


def lerp(a: int, b: int, t: float) -> int:
    return int(a + (b - a) * t)


def gradient_rect(draw, box, top, bot, vertical=True):
    x0, y0, x1, y1 = box
    if vertical:
        for y in range(y0, y1):
            t = (y - y0) / max(1, (y1 - y0 - 1))
            draw.line([(x0, y), (x1, y)], fill=(lerp(top[0], bot[0], t), lerp(top[1], bot[1], t), lerp(top[2], bot[2], t)))
    else:
        for x in range(x0, x1):
            t = (x - x0) / max(1, (x1 - x0 - 1))
            draw.line([(x, y0), (x, y1)], fill=(lerp(top[0], bot[0], t), lerp(top[1], bot[1], t), lerp(top[2], bot[2], t)))


def vertical_gradient_text(draw, xy, text, font, top, bot):
    """Approximate a vertical text gradient by drawing in stripes."""
    x, y = xy
    bbox = draw.textbbox((x, y), text, font=font)
    if bbox[2] - bbox[0] == 0:
        return
    stride = 4
    for dy in range(0, bbox[3] - bbox[1], stride):
        t = dy / max(1, (bbox[3] - bbox[1] - 1))
        col = (lerp(top[0], bot[0], t), lerp(top[1], bot[1], t), lerp(top[2], bot[2], t))
        draw.text((x, y + dy), text, font=font, fill=col)


def main():
    img = Image.new("RGB", (WIDTH, HEIGHT), BG)
    draw = ImageDraw.Draw(img)

    # Subtle radial glow centred on the brand mark
    glow = Image.new("RGBA", (WIDTH, HEIGHT), (0, 0, 0, 0))
    gd = ImageDraw.Draw(glow)
    cx, cy = int(WIDTH * 0.32), int(HEIGHT * 0.4)
    for r in range(420, 0, -8):
        alpha = int(28 * (1 - r / 420))
        gd.ellipse([cx - r, cy - r, cx + r, cy + r], fill=(164, 181, 214, alpha))
    img.paste(glow, (0, 0), glow)

    # Brand mark
    mark_size = 230
    mx, my = 160, 205
    # Background of the mark with gradient
    for dy in range(mark_size):
        t = dy / (mark_size - 1)
        col = (lerp(MARK_BG_TOP[0], MARK_BG_BOT[0], t), lerp(MARK_BG_TOP[1], MARK_BG_BOT[1], t), lerp(MARK_BG_TOP[2], MARK_BG_BOT[2], t))
        draw.line([(mx, my + dy), (mx + mark_size, my + dy)], fill=col)
    # Rounded mask: redraw rounded rect on top to ensure smooth corners.
    rounded = Image.new("RGBA", (mark_size, mark_size), (0, 0, 0, 0))
    rd = ImageDraw.Draw(rounded)
    for dy in range(mark_size):
        t = dy / (mark_size - 1)
        col = (lerp(MARK_BG_TOP[0], MARK_BG_BOT[0], t), lerp(MARK_BG_TOP[1], MARK_BG_BOT[1], t), lerp(MARK_BG_TOP[2], MARK_BG_BOT[2], t), 255)
        rd.line([(0, dy), (mark_size, dy)], fill=col)
    mask = Image.new("L", (mark_size, mark_size), 0)
    md = ImageDraw.Draw(mask)
    md.rounded_rectangle([0, 0, mark_size - 1, mark_size - 1], radius=int(mark_size * 0.31), fill=255)
    img.paste(rounded, (mx, my), mask)

    # Sheen overlay on the mark
    sheen = Image.new("RGBA", (mark_size, mark_size), (0, 0, 0, 0))
    sd = ImageDraw.Draw(sheen)
    for dy in range(mark_size):
        alpha = int(56 * max(0, 1 - dy / (mark_size * 0.55)))
        sd.line([(0, dy), (mark_size, dy)], fill=(255, 255, 255, alpha))
    img.paste(sheen, (mx, my), mask)

    # Draw the "P" inside the mark by stroking + filling the path.
    # PIL has no vector path API, so we rasterise at this size.
    P_FILL_TOP = MARK_FILL_TOP
    P_FILL_MID = MARK_FILL_MID
    P_FILL_BOT = MARK_FILL_BOT
    s = 3.6  # matches the SVG scale
    # Coordinates relative to mark top-left.
    p_bbox = [(19 * s, 14 * s), (38 * s, 40 * s)]
    # Outer stem (vertical + bowl back)
    stem = [(19 * s, 14 * s), (38 * s, 14 * s), (45 * s, 14 * s + (13 * s)), (45 * s, 14 * s + (26 * s)),
            (38 * s, 14 * s + (39 * s)), (28 * s, 14 * s + (39 * s)), (28 * s, 14 * s + (50 * s)),
            (19 * s, 14 * s + (50 * s))]
    # Inner bowl cutout
    bowl = [(28 * s, 14 * s + (22 * s)), (38 * s, 14 * s + (22 * s)),
            (43 * s, 14 * s + (27 * s)), (38 * s, 14 * s + (32 * s)),
            (28 * s, 14 * s + (32 * s))]
    # Render with a vertical gradient by drawing in stripes.
    p_stripes = Image.new("RGBA", (mark_size, mark_size), (0, 0, 0, 0))
    pd = ImageDraw.Draw(p_stripes)
    pd.polygon(stem, fill=(255, 255, 255, 255))
    # Subtract bowl (draw background-coloured polygon over)
    pd.polygon(bowl, fill=(0, 0, 0, 0))
    # Apply gradient per pixel by sampling the alpha mask.
    grad_layer = Image.new("RGBA", (mark_size, mark_size), (0, 0, 0, 0))
    gd2 = ImageDraw.Draw(grad_layer)
    for y in range(mark_size):
        t = y / (mark_size - 1)
        col = (lerp(P_FILL_TOP[0], P_FILL_BOT[0], t), lerp(P_FILL_TOP[1], P_FILL_BOT[1], t), lerp(P_FILL_TOP[2], P_FILL_BOT[2], t), 255)
        gd2.line([(0, y), (mark_size, y)], fill=col)
    out_p = Image.composite(grad_layer, Image.new("RGBA", (mark_size, mark_size), (0, 0, 0, 0)), p_stripes)
    img.paste(out_p, (mx, my), out_p)

    # Wordmark
    try:
        word_font = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 108, index=0)
        tag_font = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 32, index=0)
        sub_font = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 22, index=0)
        mono_font = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 22, index=1)
    except Exception:
        word_font = ImageFont.load_default()
        tag_font = ImageFont.load_default()
        sub_font = ImageFont.load_default()
        mono_font = ImageFont.load_default()

    vertical_gradient_text(draw, (430, 240), "promptsheon", word_font, WORDMARK_TOP, WORDMARK_BOT)
    draw.text((430, 395), "Git-native version control for AI agents", font=tag_font, fill=TAGLINE)
    draw.text((430, 445), "DAG editor  ·  Maker-checker approvals  ·  Eval suites  ·  Self-evolution", font=sub_font, fill=SUBTAG)
    draw.text((430, 535), "github.com/sachncs/promptsheon", font=mono_font, fill=SUBTAG)

    img.save(OUT, "PNG", optimize=True)
    print(f"Wrote {OUT}")


if __name__ == "__main__":
    main()
