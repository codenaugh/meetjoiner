package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework EventKit -Wl,-sectcreate,__TEXT,__info_plist,${SRCDIR}/Info.plist

#import <Foundation/Foundation.h>
#import <EventKit/EventKit.h>

static EKEventStore *sharedStore = nil;

static EKEventStore *getEventStore(void) {
	static dispatch_once_t onceToken;
	dispatch_once(&onceToken, ^{
		sharedStore = [[EKEventStore alloc] init];
	});
	return sharedStore;
}

static int requestCalendarAccess(char **errOut) {
	@autoreleasepool {
		EKEventStore *store = getEventStore();
		dispatch_semaphore_t sem = dispatch_semaphore_create(0);
		__block BOOL granted = NO;
		__block NSError *accessErr = nil;

		if (@available(macOS 14.0, *)) {
			[store requestFullAccessToEventsWithCompletion:^(BOOL g, NSError * _Nullable error) {
				granted = g;
				accessErr = error;
				dispatch_semaphore_signal(sem);
			}];
		} else {
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
			[store requestAccessToEntityType:EKEntityTypeEvent completion:^(BOOL g, NSError * _Nullable error) {
				granted = g;
				accessErr = error;
				dispatch_semaphore_signal(sem);
			}];
#pragma clang diagnostic pop
		}

		dispatch_semaphore_wait(sem, DISPATCH_TIME_FOREVER);
		if (!granted) {
			NSString *msg = accessErr ? accessErr.localizedDescription : @"Calendar access denied in System Settings > Privacy & Security > Calendars";
			*errOut = strdup([msg UTF8String]);
			return 0;
		}
		return 1;
	}
}

static char *listCalendarsJSON(void) {
	@autoreleasepool {
		EKEventStore *store = getEventStore();
		NSArray<EKCalendar *> *cals = [store calendarsForEntityType:EKEntityTypeEvent];
		NSMutableArray *arr = [NSMutableArray array];
		for (EKCalendar *cal in cals) {
			[arr addObject:@{
				@"title": cal.title ?: @"",
				@"source": cal.source.title ?: @"",
				@"id": cal.calendarIdentifier ?: @""
			}];
		}
		NSData *data = [NSJSONSerialization dataWithJSONObject:arr options:0 error:nil];
		NSString *jsonStr = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
		return strdup([jsonStr UTF8String]);
	}
}

static char *fetchEventsJSON(const char *cCalendarFilter, long long startMinUnix, long long startMaxUnix) {
	@autoreleasepool {
		EKEventStore *store = getEventStore();
		[store refreshSourcesIfNecessary];

		NSString *filter = cCalendarFilter ? [NSString stringWithUTF8String:cCalendarFilter] : @"";
		NSArray<EKCalendar *> *allCals = [store calendarsForEntityType:EKEntityTypeEvent];
		NSMutableArray<EKCalendar *> *targetCals = [NSMutableArray array];

		if (filter.length == 0 || [filter isEqualToString:@"all"] || [filter isEqualToString:@"primary"]) {
			[targetCals addObjectsFromArray:allCals];
		} else {
			for (EKCalendar *cal in allCals) {
				BOOL matchTitle = (cal.title && [cal.title localizedCaseInsensitiveContainsString:filter]);
				BOOL matchSource = (cal.source.title && [cal.source.title localizedCaseInsensitiveContainsString:filter]);
				if (matchTitle || matchSource) {
					[targetCals addObject:cal];
				}
			}
		}

		if (targetCals.count == 0) {
			return strdup("[]");
		}

		NSDate *startDate = [NSDate dateWithTimeIntervalSince1970:startMinUnix];
		NSDate *endDate = [NSDate dateWithTimeIntervalSince1970:startMaxUnix];
		NSPredicate *pred = [store predicateForEventsWithStartDate:startDate endDate:endDate calendars:targetCals];
		NSArray<EKEvent *> *events = [store eventsMatchingPredicate:pred];

		NSMutableArray *result = [NSMutableArray array];
		for (EKEvent *ev in events) {
			if (ev.isAllDay) continue;
			if (ev.status == EKEventStatusCanceled) continue;

			// Determine acceptance status
			BOOL accepted = YES;
			if (ev.hasAttendees && ev.attendees.count > 0) {
				if (ev.organizer && ev.organizer.isCurrentUser) {
					accepted = YES;
				} else {
					for (EKParticipant *p in ev.attendees) {
						if (p.isCurrentUser) {
							accepted = (p.participantStatus == EKParticipantStatusAccepted);
							break;
						}
					}
				}
			}
			if (!accepted) continue;

			// Build list of other attendee names
			NSMutableArray<NSString *> *attendeeNames = [NSMutableArray array];
			if (ev.hasAttendees) {
				for (EKParticipant *p in ev.attendees) {
					if (p.isCurrentUser) continue;
					if (p.participantType == EKParticipantTypeRoom || p.participantType == EKParticipantTypeResource) continue;
					if (p.participantStatus == EKParticipantStatusDeclined) continue;
					NSString *name = p.name;
					if (!name || name.length == 0) {
						NSString *spec = p.URL.resourceSpecifier;
						if (spec && spec.length > 0) {
							name = [[spec componentsSeparatedByString:@"@"] firstObject];
						}
					}
					if (name && name.length > 0) {
						[attendeeNames addObject:name];
					}
				}
			}

			long long startTs = (long long)[ev.startDate timeIntervalSince1970];
			NSString *uid = [NSString stringWithFormat:@"%@_%lld", ev.calendarItemIdentifier ?: @"", startTs];

			[result addObject:@{
				@"id": uid,
				@"summary": ev.title ?: @"Google Meet",
				@"start_unix": @(startTs),
				@"url": ev.URL ? (ev.URL.absoluteString ?: @"") : @"",
				@"location": ev.location ?: @"",
				@"notes": ev.notes ?: @"",
				@"attendees": attendeeNames
			}];
		}

		NSData *data = [NSJSONSerialization dataWithJSONObject:result options:0 error:nil];
		NSString *jsonStr = [[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding];
		return strdup([jsonStr UTF8String]);
	}
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unsafe"
)

var meetLinkRegex = regexp.MustCompile(`https://meet\.google\.com/[a-zA-Z0-9-]+`)

type calendarInfo struct {
	Title  string `json:"title"`
	Source string `json:"source"`
	ID     string `json:"id"`
}

type localEvent struct {
	ID        string   `json:"id"`
	Summary   string   `json:"summary"`
	StartUnix int64    `json:"start_unix"`
	URL       string   `json:"url"`
	Location  string   `json:"location"`
	Notes     string   `json:"notes"`
	Attendees []string `json:"attendees"`
}

func ensureCalendarAccess() error {
	var cErr *C.char
	ok := C.requestCalendarAccess(&cErr)
	if ok == 0 {
		defer C.free(unsafe.Pointer(cErr))
		return fmt.Errorf("%s", C.GoString(cErr))
	}
	return nil
}

func getCalendars() ([]calendarInfo, error) {
	if err := ensureCalendarAccess(); err != nil {
		return nil, err
	}
	cJSON := C.listCalendarsJSON()
	defer C.free(unsafe.Pointer(cJSON))

	var cals []calendarInfo
	if err := json.Unmarshal([]byte(C.GoString(cJSON)), &cals); err != nil {
		return nil, err
	}
	return cals, nil
}

func fetchUpcomingEvents(calendarFilter string, tMin, tMax time.Time) ([]localEvent, error) {
	cFilter := C.CString(calendarFilter)
	defer C.free(unsafe.Pointer(cFilter))

	cJSON := C.fetchEventsJSON(cFilter, C.longlong(tMin.Unix()), C.longlong(tMax.Unix()))
	defer C.free(unsafe.Pointer(cJSON))

	var events []localEvent
	if err := json.Unmarshal([]byte(C.GoString(cJSON)), &events); err != nil {
		return nil, err
	}
	return events, nil
}

func extractMeetLinkFromEvent(ev localEvent) string {
	for _, field := range []string{ev.URL, ev.Location, ev.Notes} {
		if m := meetLinkRegex.FindString(field); m != "" {
			if m != "https://meet.google.com/landing" && !strings.HasSuffix(m, "/new") {
				return m
			}
		}
	}
	return ""
}

func formatAttendeeList(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return "1:1 with " + names[0]
	case 2:
		return "with " + names[0] + " & " + names[1]
	default:
		return fmt.Sprintf("with %s, %s + %d others", names[0], names[1], len(names)-2)
	}
}
