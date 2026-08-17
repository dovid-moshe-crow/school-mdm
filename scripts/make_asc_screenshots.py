from PIL import Image, ImageDraw, ImageFont
from pathlib import Path

W, H = 1284, 2778
out = Path("mobile/asc-screenshots")
out.mkdir(parents=True, exist_ok=True)

GREEN = (11, 61, 46)
GREEN2 = (26, 107, 82)
BG = (238, 245, 241)
WHITE = (255, 255, 255)
TEXT = (18, 34, 28)
MUTED = (74, 99, 90)
BORDER = (197, 212, 206)


def font(size, bold=False):
    candidates = [
        "C:/Windows/Fonts/segoeuib.ttf" if bold else "C:/Windows/Fonts/segoeui.ttf",
        "C:/Windows/Fonts/arialbd.ttf" if bold else "C:/Windows/Fonts/arial.ttf",
        "C:/Windows/Fonts/tahoma.ttf",
    ]
    for p in candidates:
        try:
            return ImageFont.truetype(p, size)
        except Exception:
            continue
    return ImageFont.load_default()


def round_rect(draw, xy, r, fill, outline=None, width=2):
    draw.rounded_rectangle(xy, radius=r, fill=fill, outline=outline, width=width)


def save(img, name):
    path = out / name
    img.save(path)
    print(path, img.size)


# 1) Home
img = Image.new("RGB", (W, H), BG)
d = ImageDraw.Draw(img)
d.rectangle((0, 0, W, 320), fill=GREEN)
d.text((60, 90), "9:41", fill=WHITE, font=font(34, True))
d.text((W - 420, 170), "KFilter", fill=WHITE, font=font(64, True))
d.text((W - 520, 250), "Student portal", fill=(200, 225, 210), font=font(36))

y = 380
cards = [
    ("Requests", "Track status of your school requests"),
    ("Allowed apps", "Apps available on this device"),
    ("Credits", "Balance for temporary access"),
]
for title, body in cards:
    round_rect(d, (60, y, W - 60, y + 220), 28, WHITE, BORDER, 3)
    d.text((100, y + 50), title, fill=GREEN, font=font(44, True))
    d.text((100, y + 120), body, fill=MUTED, font=font(32))
    y += 260
d.text((W // 2 - 180, H - 180), "nanok.kfilter.net", fill=MUTED, font=font(28))
save(img, "01-home-1284x2778.png")

# 2) Requests
img = Image.new("RGB", (W, H), BG)
d = ImageDraw.Draw(img)
d.rectangle((0, 0, W, 280), fill=GREEN)
d.text((60, 90), "9:41", fill=WHITE, font=font(34, True))
d.text((W - 360, 170), "Requests", fill=WHITE, font=font(56, True))
items = [
    ("Approved", GREEN2, "App access — WhatsApp"),
    ("Pending", (180, 120, 40), "General — install help"),
    ("Denied", (140, 50, 40), "Website — example.com"),
]
y = 340
for status, color, title in items:
    round_rect(d, (60, y, W - 60, y + 240), 28, WHITE, BORDER, 3)
    d.text((100, y + 50), title, fill=TEXT, font=font(36, True))
    round_rect(d, (100, y + 130, 320, y + 190), 16, color)
    d.text((130, y + 140), status, fill=WHITE, font=font(30, True))
    y += 280
save(img, "02-requests-1284x2778.png")

# 3) Updates / notification
img = Image.new("RGB", (W, H), BG)
d = ImageDraw.Draw(img)
d.rectangle((0, 0, W, 280), fill=GREEN)
d.text((60, 90), "9:41", fill=WHITE, font=font(34, True))
d.text((W - 340, 170), "Updates", fill=WHITE, font=font(56, True))
round_rect(d, (80, 360, W - 80, 560), 32, WHITE, BORDER, 3)
d.text((120, 400), "KFilter", fill=GREEN, font=font(36, True))
d.text((120, 460), "Your request was approved", fill=TEXT, font=font(34))
round_rect(d, (80, 620, W - 80, 980), 32, WHITE, BORDER, 3)
d.text((120, 680), "Message from school", fill=GREEN, font=font(40, True))
d.text((120, 780), "You can use the app until", fill=MUTED, font=font(34))
d.text((120, 840), "the end of the day.", fill=MUTED, font=font(34))
save(img, "03-updates-1284x2778.png")
