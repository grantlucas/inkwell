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
hidden. That last line counts every event left out for lack of room, and takes
the place of an event when there is no room for both. Cancelled and declined
events are never counted.
_Avoid_: Event column, overflow marker

## Calendar

**Feed**:
One subscribed calendar, together with the cleanup its events need before they
are shown.
_Avoid_: Calendar, subscription, source

**Feed owner**:
The person whose calendar a feed is. A feed has at most one; a feed with none
has no declined events.
_Avoid_: Self, user, me

**Cancelled event**:
An event its organizer has called off. It is off for everyone and is never
shown.
_Avoid_: Deleted event, removed event

**Declined event**:
An event the feed owner has said no to. It still happens for everyone else, but
is never shown. Each occurrence of a repeating event is declined on its own.
_Avoid_: Rejected event, hidden event

**Unavailable feed**:
A feed with nothing to show: its fetch failed and there is no earlier good copy
of it. A widget says the calendar is unavailable rather than draw the day as
free. A feed that fails while a copy is cached shows that copy and is not
unavailable.
_Avoid_: Calendar down, offline feed, missing calendar

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
hourly weather lane beside it. It is composed from widgets: the fuzzy clock
across the top, the day-timeline widget (agenda and weather lane) on the left,
and today-weather above weather-ahead on the right.
_Avoid_: Daily planner, schedule view

**Weather lane**:
The strip of the day-timeline that carries hourly precipitation and temperature
alongside the agenda hours.
_Avoid_: Weather column, side chart

**All-day strip**:
The lines above the day-timeline's grid listing today's all-day events, and
timed events that run through the whole of today.
_Avoid_: All-day row, banner

**Day badge**:
The head of one day on a screen: its date and its weather in brief (condition,
high and low). Day screens draw each day as a day badge, a combined chart and
an event list, and each of the three can also be placed as a widget of its own.
_Avoid_: Day header, date block, weather badge

**Fuzzy clock**:
The time written in words, such as "twenty to eleven".
_Avoid_: Word clock, text clock

## Display discipline

**Burn-in**:
Ghosting of an image that sits in the same place for a long time. Large filled
black areas that always land in the same spot invite it.
_Avoid_: Image retention
