# Python Driver Architecture

This document explains the architecture of the Waveshare Python driver for the
7.5" e-Paper V2, broken into its two main modules.

## Module 1: `epdconfig.py` — Hardware Abstraction Layer

This module provides a platform-agnostic interface to the hardware. It detects
the platform (Raspberry Pi, Jetson Nano, or Sunrise X3) by reading
`/proc/cpuinfo` and instantiates the appropriate class.

### Platform Detection

```python
# Pseudocode of detection logic
if "Raspberry" in /proc/cpuinfo:
    implementation = RaspberryPi()
elif exists("/sys/bus/platform/drivers/gpio-x3"):
    implementation = SunriseX3()
else:
    implementation = JetsonNano()
```

### RaspberryPi Class — Key Details

**GPIO Setup (using `gpiozero`):**

```python
RST_PIN  = 17   # Reset — active low pulse to reset the controller
DC_PIN   = 25   # Data/Command — LOW for command bytes, HIGH for data bytes
CS_PIN   = 8    # Chip Select (directly wired to CE0 from SPI)
BUSY_PIN = 24   # Busy — LOW means display is processing, HIGH means idle
PWR_PIN  = 18   # Power control for the display
```

**SPI Setup:**

```python
import spidev
SPI = spidev.SpiDev()
SPI.open(0, 0)           # Bus 0, Device 0 (/dev/spidev0.0)
SPI.max_speed_hz = 4000000  # 4 MHz
SPI.mode = 0b00          # CPOL=0, CPHA=0
```

**Core Methods Exported:**

```python
def digital_write(pin, value)  # Set a GPIO pin HIGH or LOW
def digital_read(pin)          # Read a GPIO pin state
def spi_writebyte(data)        # Send a list of bytes over SPI (small payloads)
# SPI.writebytes2(data)        # Send large byte arrays (used via SPI object directly)
def delay_ms(ms)               # Sleep for milliseconds
def module_init()              # Initialize SPI + GPIO, power on display
def module_exit(cleanup=True)  # Power off display, cleanup GPIO
```

### Native Library Fallback

The module attempts to load a native C shared library
(`DEV_Config_64.so` or `DEV_Config_32.so`) for potentially faster GPIO
operations. If loading fails, it falls back to pure Python `gpiozero` +
`spidev`.

## Module 2: `epd7in5_V2.py` — Display Driver

This is the display-specific driver. It contains all the initialization
sequences, buffer conversion logic, and display commands for the 7.5" V2.

### Class Structure

```python
class EPD:
    width = 800
    height = 480

    # Pin references (from epdconfig)
    reset_pin  = epdconfig.RST_PIN
    dc_pin     = epdconfig.DC_PIN
    busy_pin   = epdconfig.BUSY_PIN
    cs_pin     = epdconfig.CS_PIN

    # Grayscale constants
    GRAY1 = 0xFF  # White
    GRAY2 = 0xC0
    GRAY3 = 0x80  # Gray
    GRAY4 = 0x00  # Black
```

### Communication Primitives

The driver communicates with the e-Paper controller IC via SPI using two
fundamental operations:

```python
def send_command(self, command):
    """Send a command byte. DC pin is pulled LOW."""
    digital_write(DC, LOW)
    digital_write(CS, LOW)
    spi_writebyte([command])
    digital_write(CS, HIGH)

def send_data(self, data):
    """Send a single data byte. DC pin is pulled HIGH."""
    digital_write(DC, HIGH)
    digital_write(CS, LOW)
    spi_writebyte([data])
    digital_write(CS, HIGH)

def send_data2(self, data):
    """Send large data payload (bulk transfer). DC pin HIGH."""
    digital_write(DC, HIGH)
    digital_write(CS, LOW)
    SPI.writebytes2(data)   # Efficient bulk SPI write
    digital_write(CS, HIGH)
```

The **DC (Data/Command) pin** is the key differentiator:

- **DC = LOW:** The byte on the SPI bus is interpreted as a command
- **DC = HIGH:** The byte(s) on the SPI bus are interpreted as data/parameters

### Initialization Modes

The driver supports four initialization modes, each configuring the display
controller differently:

#### `init()` — Standard Full Refresh

Used for normal full-screen updates with best image quality.

```python
def init(self):
    self.reset()                # Hardware reset

    self.send_command(0x06)     # Booster soft start
    self.send_data(0x17)        # Phase A: strength, duration
    self.send_data(0x17)        # Phase B
    self.send_data(0x28)        # Phase C
    self.send_data(0x17)

    self.send_command(0x01)     # Power setting
    self.send_data(0x07)        # VDS_EN, VDG_EN
    self.send_data(0x07)        # VCOM_HV, VGHL_LV
    self.send_data(0x28)        # VDH
    self.send_data(0x17)        # VDL

    self.send_command(0x04)     # Power on
    self.ReadBusy()             # Wait for power-on complete

    self.send_command(0x00)     # Panel setting
    self.send_data(0x1F)        # KW mode, scan up, shift right, booster on

    self.send_command(0x61)     # Resolution setting
    self.send_data(0x03)        # 800 >> 8
    self.send_data(0x20)        # 800 & 0xFF
    self.send_data(0x01)        # 480 >> 8
    self.send_data(0xE0)        # 480 & 0xFF

    self.send_command(0x15)     # Dual SPI mode
    self.send_data(0x00)        # Disabled

    self.send_command(0x50)     # VCOM and data interval
    self.send_data(0x10)
    self.send_data(0x07)

    self.send_command(0x60)     # TCON setting
    self.send_data(0x22)
```

#### `init_fast()` — Fast Full Refresh (~1.5s)

Trades image quality for speed. Uses enhanced driving parameters:

```python
def init_fast(self):
    self.reset()
    # ... panel + VCOM settings ...
    self.send_command(0xE0)     # Cascade setting
    self.send_data(0x02)
    self.send_command(0xE5)     # Force temperature
    self.send_data(0x5A)        # Temperature value for fast refresh
```

#### `init_part()` — Partial Refresh (~0.4s)

For updating a region of the screen without redrawing everything:

```python
def init_part(self):
    self.reset()
    # ... panel settings ...
    self.send_command(0xE0)     # Cascade setting
    self.send_data(0x02)
    self.send_command(0xE5)     # Force temperature
    self.send_data(0x6E)        # Temperature value for partial refresh
```

#### `init_4Gray()` — 4-Level Grayscale

Enables 4 shades: white (0xFF), light gray (0xC0), dark gray (0x80), black
(0x00):

```python
def init_4Gray(self):
    self.reset()
    # ... similar to init_fast but with different temperature ...
    self.send_command(0xE5)
    self.send_data(0x5F)        # Temperature for grayscale mode
```

### Busy Wait

The display signals completion by pulling the BUSY pin HIGH:

```python
def ReadBusy(self):
    self.send_command(0x71)     # Get status command
    while digital_read(BUSY_PIN) == 0:
        self.send_command(0x71)
    delay_ms(20)
```

### Hardware Reset

A pulse sequence resets the display controller:

```python
def reset(self):
    digital_write(RST, HIGH)
    delay_ms(20)
    digital_write(RST, LOW)     # Pull low to reset
    delay_ms(2)
    digital_write(RST, HIGH)    # Release
    delay_ms(20)
```
