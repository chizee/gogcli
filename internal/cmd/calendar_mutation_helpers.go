package cmd

import (
	"context"
	"os"

	"google.golang.org/api/calendar/v3"

	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

type calendarMutationContext struct {
	ctx        context.Context
	u          *ui.UI
	svc        *calendar.Service
	calendarID string
}

type calendarInsertOptions struct {
	sendUpdates         string
	conferenceVersion1  bool
	supportsAttachments bool
}

func newCalendarMutationContext(ctx context.Context, flags *RootFlags, calendarID string) (*calendarMutationContext, error) {
	_, svc, err := requireCalendarService(ctx, flags)
	if err != nil {
		return nil, err
	}
	resolvedCalendarID, err := resolveCalendarID(ctx, svc, calendarID)
	if err != nil {
		return nil, err
	}
	return &calendarMutationContext{
		ctx:        ctx,
		u:          ui.FromContext(ctx),
		svc:        svc,
		calendarID: resolvedCalendarID,
	}, nil
}

func (m *calendarMutationContext) insertEvent(event *calendar.Event, opts calendarInsertOptions) (*calendar.Event, error) {
	call := m.svc.Events.Insert(m.calendarID, event).Context(m.ctx)
	if opts.sendUpdates != "" {
		call = call.SendUpdates(opts.sendUpdates)
	}
	if opts.conferenceVersion1 {
		call = call.ConferenceDataVersion(1)
	}
	if opts.supportsAttachments {
		call = call.SupportsAttachments(true)
	}
	return call.Do()
}

func (m *calendarMutationContext) patchEvent(eventID string, patch *calendar.Event, sendUpdates string) (*calendar.Event, error) {
	call := m.svc.Events.Patch(m.calendarID, eventID, patch).Context(m.ctx)
	if sendUpdates != "" {
		call = call.SendUpdates(sendUpdates)
	}
	return call.Do()
}

func (m *calendarMutationContext) deleteEvent(eventID, sendUpdates string) error {
	call := m.svc.Events.Delete(m.calendarID, eventID).Context(m.ctx)
	if sendUpdates != "" {
		call = call.SendUpdates(sendUpdates)
	}
	return call.Do()
}

func (m *calendarMutationContext) writeEvent(event *calendar.Event) error {
	tz, loc, _ := getCalendarLocation(m.ctx, m.svc, m.calendarID)
	if outfmt.IsJSON(m.ctx) {
		return outfmt.WriteJSON(m.ctx, os.Stdout, map[string]any{"event": wrapEventWithDaysWithTimezone(event, tz, loc)})
	}
	printCalendarEventWithTimezone(m.u, event, tz, loc)
	return nil
}
