# SPI Command Reference

This documents the SPI commands used by the 7.5" e-Paper V2 controller IC, as
observed in the Waveshare Python driver.

## Communication Protocol

All communication uses SPI Mode 0 (CPOL=0, CPHA=0) at 4 MHz, MSB first.

The **DC (Data/Command) pin** determines how the controller interprets each
byte:

- **DC = LOW:** Byte is a command register address
- **DC = HIGH:** Byte is data (parameter or payload for the previous command)

Typical sequence:

```text
DC=LOW  → SPI write command byte
DC=HIGH → SPI write data byte(s) for that command
```

## Command Table

Commands are listed in the order they appear in the driver's `init()` sequence.

### Power and Startup

<!-- markdownlint-disable MD013 -->
| Command | Name | Data | Description |
|---------|------|------|-------------|
| `0x06` | Booster Soft Start | `0x17, 0x17, 0x28, 0x17` | Configure booster circuit phases A/B/C |
| `0x01` | Power Setting | `0x07, 0x07, 0x28, 0x17` | VDS, VDG enable; VCOM_HV, VGHL levels; VDH/VDL voltages |
| `0x04` | Power On | (none) | Turn on DC-DC converter. Wait for BUSY=HIGH after |
| `0x02` | Power Off | (none) | Turn off DC-DC converter. Wait for BUSY=HIGH after |
| `0x07` | Deep Sleep | `0xA5` | Enter deep sleep. `0xA5` is a check code |
<!-- markdownlint-enable MD013 -->

### Panel Configuration

<!-- markdownlint-disable MD013 -->
| Command | Name | Data | Description |
|---------|------|------|-------------|
| `0x00` | Panel Setting | `0x1F` | KW mode, scan up, shift right, booster on, no reset |
| `0x61` | Resolution Setting | `0x03, 0x20, 0x01, 0xE0` | Width=800 (0x0320), Height=480 (0x01E0) |
| `0x15` | Dual SPI | `0x00` | Dual SPI disabled |
| `0x50` | VCOM and Data Interval | `0x10, 0x07` | Border and data output settings |
| `0x60` | TCON Setting | `0x22` | Non-overlap period |
<!-- markdownlint-enable MD013 -->

### Display Data

<!-- markdownlint-disable MD013 -->
| Command | Name | Data | Description |
|---------|------|------|-------------|
| `0x10` | Data Start Transmission 1 | 48000 bytes | "Old" frame data (previous image) |
| `0x13` | Data Start Transmission 2 | 48000 bytes | "New" frame data (current image) |
| `0x12` | Display Refresh | (none) | Trigger the electrophoretic refresh cycle |
| `0x71` | Get Status | (none) | Read BUSY status. Used in polling loop |
<!-- markdownlint-enable MD013 -->

### Partial Refresh

<!-- markdownlint-disable MD013 -->
| Command | Name | Data | Description |
|---------|------|------|-------------|
| `0x91` | Partial In | (none) | Enter partial update mode |
| `0x90` | Partial Window | 9 bytes | Define the update region (Xstart, Xend, Ystart, Yend, scan) |
| `0x92` | Partial Out | (none) | Exit partial update mode (not used in this driver) |
<!-- markdownlint-enable MD013 -->

### Fast/Grayscale Mode

<!-- markdownlint-disable MD013 -->
| Command | Name | Data | Description |
|---------|------|------|-------------|
| `0xE0` | Cascade Setting | `0x02` | Enable temperature forcing |
| `0xE5` | Force Temperature | varies | `0x5A`=fast, `0x6E`=partial, `0x5F`=4gray |
<!-- markdownlint-enable MD013 -->

### Sleep Sequence

<!-- markdownlint-disable MD013 -->
| Command | Name | Data | Description |
|---------|------|------|-------------|
| `0x50` | VCOM Setting | `0xF7` | Pre-sleep VCOM configuration |
| `0x02` | Power Off | (none) | DC-DC off, wait BUSY |
| `0x07` | Deep Sleep | `0xA5` | Enter deep sleep mode |
<!-- markdownlint-enable MD013 -->

## Data Format Details

### 1-Bit Mode (Black/White)

Each byte encodes 8 horizontal pixels:

```text
Bit 7 (MSB) = leftmost pixel
Bit 0 (LSB) = rightmost pixel

After getbuffer() XOR inversion:
  1 = black pixel
  0 = white pixel
```

Buffer size: `800 * 480 / 8 = 48,000 bytes`

### 4-Gray Mode

Each byte encodes 4 horizontal pixels (2 bits each):

```text
Bits 7-6 = pixel 0 (leftmost)
Bits 5-4 = pixel 1
Bits 3-2 = pixel 2
Bits 1-0 = pixel 3 (rightmost)
```

Buffer size: `800 * 480 / 4 = 96,000 bytes`

This buffer is then split into two 48,000-byte planes for commands 0x10 and
0x13.

## Timing Constraints

- After `0x04` (Power On): wait for BUSY=HIGH (typically 100ms+ delay first)
- After `0x12` (Refresh): wait for BUSY=HIGH (full refresh takes ~4 seconds)
- Between full refreshes: minimum 180 seconds recommended
- Deep sleep wake: requires hardware reset sequence
