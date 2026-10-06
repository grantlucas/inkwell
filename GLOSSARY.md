# Inkwell

A dashboard for a small e-paper panel, assembled from widgets that are
arranged into screens and shown in rotation.

## Language

**Panel**:
The physical e-paper display Inkwell draws on.
_Avoid_: Display, monitor, device (outside hardware docs)

**Screen**:
A named entry in the rotation, made of one or more widgets. The panel shows one
screen at a time.
_Avoid_: View, page, option, layout

**Widget**:
A placeable piece of a screen, with its own bounds and refresh cadence.
_Avoid_: Component, module, panel (a Panel is hardware)

**Bounds**:
The rectangle of the panel a widget draws into.
_Avoid_: Region, frame, area

**Refresh cadence**:
How often a widget may cause the panel to refresh. Set per widget, never per
screen.
_Avoid_: Update interval, polling rate

**Today**:
The current calendar day. It is shown by position (first column or first row),
not by a distinct fill or outline.
_Avoid_: Current day highlight

**Day data**:
A day's date, whether it is Today, its events and its forecast, as one widget
sees them. A day the forecast doesn't reach has no forecast, rather than an
empty one.
_Avoid_: Day column, grid day

**Event list**:
The lines a widget draws for a day's events, ending in "+N MORE" when some are
hidden. That last line counts every event not shown, and takes the place of an
event when there is no room for both.
_Avoid_: Event column, overflow marker

## Weather charts

**Combined chart**:
The hourly chart that draws precipitation bars with the temperature line over
them, so a dry day still shows temperature.
_Avoid_: Hourly graph, weather graph

**Shared temperature range**:
One temperature scale used by every day shown on a screen, so charts can be
compared across days.
_Avoid_: Global scale

**Now marker**:
The mark on an hourly chart or timeline showing the current time.
_Avoid_: Cursor, current-time line

## Screens and widgets

**Day-timeline**:
A screen that centres on today: the agenda placed by time of day, with the
hourly weather lane beside it.
_Avoid_: Daily planner, schedule view

**Weather lane**:
The strip of the day-timeline that carries hourly precipitation and temperature
alongside the agenda hours.
_Avoid_: Weather column, side chart

**All-day strip**:
The lines above the day-timeline's grid listing today's all-day events, and
timed events that run through the whole of today.
_Avoid_: All-day row, banner

**Fuzzy clock**:
The time written in words, such as "twenty to eleven".
_Avoid_: Word clock, text clock

## Display discipline

**Burn-in**:
Ghosting of an image that sits in the same place for a long time. Large filled
black areas that always land in the same spot invite it.
_Avoid_: Image retention
