# Rendering Pipeline: From Pixels to E-Ink

This document explains exactly how an image goes from Python code to physical
pixels on the e-ink display.

## Overview

```text
PIL Image (800x480)
    │
    ▼
getbuffer() — Convert to 1-bit packed bytes
    │
    ▼
display() — Send via SPI commands
    │
    ▼
e-Paper controller IC — Drives electrophoretic particles
    │
    ▼
Physical pixels update
```

## Step 1: Create an Image with PIL

All rendering starts with a PIL `Image` object. The display is 800x480 pixels
in 1-bit black/white mode:

```python
from PIL import Image, ImageDraw, ImageFont

# Create a blank white image
# Mode '1' = 1-bit pixels (black or white)
# 255 = white background
image = Image.new('1', (800, 480), 255)

# Get a drawing context
draw = ImageDraw.Draw(image)

# Draw shapes and text
draw.text((10, 10), 'Hello World', font=font24, fill=0)  # 0 = black
draw.rectangle((50, 50, 200, 150), outline=0)
draw.line((0, 0, 100, 100), fill=0)
```

Key point: `fill=0` means black, `fill=255` (or omitted) means white. This is
standard PIL convention.

## Step 2: Convert Image to Display Buffer (`getbuffer`)

The `getbuffer()` method converts a PIL Image into the byte format the display
controller expects:

```python
def getbuffer(self, image):
    # Handle rotation if needed
    if image_is_rotated:
        image = image.rotate(90, expand=True)

    # Convert to 1-bit mode
    img = image.convert('1')

    # Extract raw bytes — PIL packs 8 pixels per byte
    buf = bytearray(img.tobytes('raw'))

    # INVERT all bytes — display uses opposite polarity
    # PIL: 0=black, 255=white
    # Display: 0xFF=black pixel, 0x00=white pixel
    for i in range(len(buf)):
        buf[i] ^= 0xFF

    return buf
```

### Buffer Format

- **Size:** `(800 / 8) * 480 = 48,000 bytes`
- **Packing:** 8 pixels per byte, MSB first
- **Polarity (after XOR inversion):**
  - Bit = 1 → black pixel
  - Bit = 0 → white pixel
- **Ordering:** Row-major, left-to-right, top-to-bottom

Example: The first byte of the buffer contains pixels (0,0) through (7,0):

```text
Byte 0:  [px(0,0)] [px(1,0)] [px(2,0)] ... [px(7,0)]
         MSB                                   LSB
Byte 1:  [px(8,0)] [px(9,0)] ...            [px(15,0)]
...
Byte 99: [px(792,0)] ... [px(799,0)]    ← End of row 0
Byte 100: [px(0,1)] ...                  ← Start of row 1
```

## Step 3: Send Buffer to Display (`display`)

The `display()` method writes the buffer to the e-Paper controller IC:

```python
def display(self, image):
    Width = self.width // 8   # 100 bytes per row
    Height = self.height      # 480 rows

    # Create INVERTED copy for the "old" image buffer
    image1 = [0xFF] * (self.width * self.height // 8)
    for j in range(Height):
        for i in range(Width):
            image1[i + j * Width] = ~image[i + j * Width]

    # Command 0x10: Write "old" data (previous frame)
    self.send_command(0x10)
    self.send_data2(image1)

    # Command 0x13: Write "new" data (current frame)
    self.send_command(0x13)
    self.send_data2(image)

    # Command 0x12: Trigger display refresh
    self.send_command(0x12)
    delay_ms(100)
    self.ReadBusy()          # Wait for refresh to complete
```

### Why Two Buffers?

E-ink displays use **electrophoretic** technology. The controller compares the
"old" frame and "new" frame to determine which particles need to move:

- Pixels changing from white to black: Apply forward voltage
- Pixels changing from black to white: Apply reverse voltage
- Pixels staying the same: No voltage applied (reduces ghosting)

For a full refresh from a clean state, the "old" buffer is set to the inverse
of the new image, forcing every pixel to be driven.

### SPI Command Sequence

```text
1. DC=LOW,  SPI: [0x10]           ← "Write old data" command
2. DC=HIGH, SPI: [48000 bytes]    ← Old frame buffer (bulk transfer)
3. DC=LOW,  SPI: [0x13]           ← "Write new data" command
4. DC=HIGH, SPI: [48000 bytes]    ← New frame buffer (bulk transfer)
5. DC=LOW,  SPI: [0x12]           ← "Display refresh" command
6. Poll BUSY pin until HIGH       ← Wait for physical refresh
```

## Step 4: Partial Refresh (`display_Partial`)

Partial refresh updates only a rectangular region of the screen:

```python
def display_Partial(self, Image, Xstart, Ystart, Xend, Yend):
    # Align X coordinates to byte boundaries (multiples of 8)
    Xstart = Xstart // 8 * 8
    Xend = Xend // 8 * 8

    Width = (Xend - Xstart) // 8
    Height = Yend - Ystart

    # Set VCOM interval for partial mode
    self.send_command(0x50)
    self.send_data(0xA9)
    self.send_data(0x07)

    # Enter partial mode
    self.send_command(0x91)

    # Set partial window coordinates
    self.send_command(0x90)
    self.send_data(Xstart // 256)       # X start high byte
    self.send_data(Xstart % 256)        # X start low byte
    self.send_data((Xend-1) // 256)     # X end high byte
    self.send_data((Xend-1) % 256)      # X end low byte
    self.send_data(Ystart // 256)       # Y start high byte
    self.send_data(Ystart % 256)        # Y start low byte
    self.send_data((Yend-1) // 256)     # Y end high byte
    self.send_data((Yend-1) % 256)      # Y end low byte
    self.send_data(0x01)                # Scan inside partial window

    # Send only the partial region data (inverted)
    self.send_command(0x13)
    self.send_data2(inverted_partial_image)

    # Trigger refresh
    self.send_command(0x12)
    self.ReadBusy()
```

### Partial Refresh Constraints

1. **X coordinates must be byte-aligned** (multiples of 8)
2. **Requires `init_part()` first** — sets the controller to partial mode
3. **Must do a full refresh periodically** — after ~10 partial refreshes,
   ghosting artifacts accumulate and only a full refresh can clear them
4. **Only sends one buffer** (command 0x13) — the controller uses its internal
   memory as the "old" frame

## Step 5: Clear Screen

```python
def Clear(self):
    # Old buffer: all white (0xFF)
    self.send_command(0x10)
    self.send_data2([0xFF] * 48000)

    # New buffer: all white (0x00 = white in display polarity)
    self.send_command(0x13)
    self.send_data2([0x00] * 48000)

    # Refresh
    self.send_command(0x12)
    self.ReadBusy()
```

## Step 6: Sleep Mode

```python
def sleep(self):
    self.send_command(0x50)    # VCOM setting
    self.send_data(0xF7)

    self.send_command(0x02)    # Power off
    self.ReadBusy()

    self.send_command(0x07)    # Deep sleep
    self.send_data(0xA5)       # Check code for deep sleep

    delay_ms(2000)
    module_exit()              # Cleanup GPIO
```

## 4-Level Grayscale

When using `init_4Gray()`, the display supports 4 shades. The buffer format
changes from 1 bit per pixel to 2 bits per pixel:

<!-- markdownlint-disable MD013 -->
| Value | Shade | 2-bit code |
|-------|-------|------------|
| 0xFF | White | 00 |
| 0xC0 | Light gray | 01 |
| 0x80 | Dark gray | 10 |
| 0x00 | Black | 11 |
<!-- markdownlint-enable MD013 -->

The `getbuffer_4Gray()` method packs 4 pixels into each byte (2 bits each).
The `display_4Gray()` method then splits this into two separate bit planes sent
to commands 0x10 and 0x13 respectively — each plane carries one bit of the
2-bit grayscale value per pixel.

## Complete Example Workflow

```python
# 1. Initialize
epd = epd7in5_V2.EPD()
epd.init()
epd.Clear()

# 2. Create image
image = Image.new('1', (800, 480), 255)
draw = ImageDraw.Draw(image)
draw.text((10, 10), 'Dashboard', font=font, fill=0)
draw.rectangle((0, 0, 799, 479), outline=0)

# 3. Convert and display (full refresh)
epd.display(epd.getbuffer(image))

# 4. Switch to partial mode for updates
epd.init_part()

# 5. Update a region
draw.rectangle((10, 100, 200, 150), fill=255)  # Clear region
draw.text((10, 100), '12:34:56', font=font, fill=0)
epd.display_Partial(epd.getbuffer(image), 0, 0, 800, 480)

# 6. After ~10 partial updates, do a full refresh
epd.init()
epd.display(epd.getbuffer(image))

# 7. Sleep when done
epd.sleep()
```
