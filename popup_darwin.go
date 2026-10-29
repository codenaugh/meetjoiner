package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore

#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>

typedef struct {
	int action; // 0 = dismiss, 1 = join, 2 = snooze
	int audio_on;
	int video_on;
} PopupResult;

// --- Close Icon Button (Top-right 'X') ---

@interface CloseIconButton : NSView
@property (nonatomic, copy) void (^onClick)(void);
@property (nonatomic, assign) BOOL isHovered;
@property (nonatomic, strong) NSImageView *iconView;
- (instancetype)initWithFrame:(NSRect)frame onClick:(void (^)(void))onClick;
@end

@implementation CloseIconButton

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
	return YES;
}

- (instancetype)initWithFrame:(NSRect)frame onClick:(void (^)(void))onClick {
	self = [super initWithFrame:frame];
	if (self) {
		_onClick = [onClick copy];
		self.wantsLayer = YES;
		self.layer.cornerRadius = frame.size.width / 2.0;

		NSTrackingArea *area = [[NSTrackingArea alloc] initWithRect:self.bounds
		                                                    options:NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways | NSTrackingInVisibleRect
		                                                      owner:self
		                                                   userInfo:nil];
		[self addTrackingArea:area];

		_iconView = [[NSImageView alloc] initWithFrame:NSMakeRect((frame.size.width - 12) / 2.0, (frame.size.height - 12) / 2.0, 12, 12)];
		NSImage *img = [NSImage imageWithSystemSymbolName:@"xmark" accessibilityDescription:@"Dismiss"];
		NSImageSymbolConfiguration *cfg = [NSImageSymbolConfiguration configurationWithPointSize:10 weight:NSFontWeightBold];
		_iconView.image = [img imageWithSymbolConfiguration:cfg];
		[self addSubview:_iconView];

		[self updateAppearance];
	}
	return self;
}

- (void)updateAppearance {
	if (_isHovered) {
		self.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.16].CGColor;
		_iconView.contentTintColor = [NSColor colorWithWhite:0.95 alpha:1.0];
	} else {
		self.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.06].CGColor;
		_iconView.contentTintColor = [NSColor colorWithWhite:0.55 alpha:1.0];
	}
}

- (void)mouseEntered:(NSEvent *)event {
	_isHovered = YES;
	[self updateAppearance];
}

- (void)mouseExited:(NSEvent *)event {
	_isHovered = NO;
	[self updateAppearance];
}

- (void)mouseDown:(NSEvent *)event {
	if (_onClick) {
		_onClick();
	}
}

- (void)resetCursorRects {
	[self addCursorRect:self.bounds cursor:[NSCursor pointingHandCursor]];
}

@end

// --- Toggle Card Button (Mic / Camera Tile with Hover, Press & Keycap Badge) ---

@interface ToggleCardButton : NSView
@property (nonatomic, assign) BOOL isOn;
@property (nonatomic, assign) BOOL isHovered;
@property (nonatomic, assign) BOOL isPressed;
@property (nonatomic, strong) NSString *onSymbol;
@property (nonatomic, strong) NSString *offSymbol;
@property (nonatomic, strong) NSString *titleText;
@property (nonatomic, strong) NSImageView *iconView;
@property (nonatomic, strong) NSTextField *titleLabel;
@property (nonatomic, strong) NSTextField *stateLabel;
@property (nonatomic, strong) NSView *keycapView;
@property (nonatomic, strong) NSTextField *keycapLabel;
- (instancetype)initWithFrame:(NSRect)frame
                        title:(NSString *)title
                     onSymbol:(NSString *)onSym
                    offSymbol:(NSString *)offSym
                       keycap:(NSString *)keycap
                    initialOn:(BOOL)initialOn;
- (void)toggle;
- (void)updateAppearance;
@end

@implementation ToggleCardButton

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
	return YES;
}

- (instancetype)initWithFrame:(NSRect)frame
                        title:(NSString *)title
                     onSymbol:(NSString *)onSym
                    offSymbol:(NSString *)offSym
                       keycap:(NSString *)keycap
                    initialOn:(BOOL)initialOn {
	self = [super initWithFrame:frame];
	if (self) {
		_isOn = initialOn;
		_titleText = title;
		_onSymbol = onSym;
		_offSymbol = offSym;

		self.wantsLayer = YES;
		self.layer.cornerRadius = 13.0;
		self.layer.borderWidth = 1.0;

		NSTrackingArea *area = [[NSTrackingArea alloc] initWithRect:self.bounds
		                                                    options:NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways | NSTrackingInVisibleRect
		                                                      owner:self
		                                                   userInfo:nil];
		[self addTrackingArea:area];

		// Subtle keycap badge in top-right of tile (e.g. "M" or "V")
		_keycapView = [[NSView alloc] initWithFrame:NSMakeRect(frame.size.width - 26, frame.size.height - 24, 18, 16)];
		_keycapView.wantsLayer = YES;
		_keycapView.layer.cornerRadius = 4.5;
		_keycapView.layer.borderWidth = 0.8;
		[self addSubview:_keycapView];

		_keycapLabel = [NSTextField labelWithString:keycap];
		_keycapLabel.frame = NSMakeRect(0, 1, 18, 14);
		_keycapLabel.alignment = NSTextAlignmentCenter;
		_keycapLabel.font = [NSFont monospacedSystemFontOfSize:9.5 weight:NSFontWeightBold];
		[_keycapView addSubview:_keycapLabel];

		_iconView = [[NSImageView alloc] initWithFrame:NSMakeRect((frame.size.width - 28) / 2.0, 50, 28, 28)];
		_iconView.imageScaling = NSImageScaleProportionallyUpOrDown;
		[self addSubview:_iconView];

		_titleLabel = [NSTextField labelWithString:title];
		_titleLabel.frame = NSMakeRect(8, 26, frame.size.width - 16, 18);
		_titleLabel.alignment = NSTextAlignmentCenter;
		_titleLabel.font = [NSFont systemFontOfSize:12.5 weight:NSFontWeightSemibold];
		_titleLabel.textColor = [NSColor colorWithWhite:0.96 alpha:1.0];
		[self addSubview:_titleLabel];

		_stateLabel = [NSTextField labelWithString:@""];
		_stateLabel.frame = NSMakeRect(8, 9, frame.size.width - 16, 15);
		_stateLabel.alignment = NSTextAlignmentCenter;
		_stateLabel.font = [NSFont systemFontOfSize:11 weight:NSFontWeightMedium];
		[self addSubview:_stateLabel];

		[self updateAppearance];
	}
	return self;
}

- (void)toggle {
	_isOn = !_isOn;
	CATransition *transition = [CATransition animation];
	transition.duration = 0.14;
	transition.type = kCATransitionFade;
	[self.layer addAnimation:transition forKey:@"toggleFade"];
	[self updateAppearance];
}

- (void)updateAppearance {
	NSString *symName = _isOn ? _onSymbol : _offSymbol;
	NSImage *img = [NSImage imageWithSystemSymbolName:symName accessibilityDescription:_titleText];
	NSImageSymbolConfiguration *cfg = [NSImageSymbolConfiguration configurationWithPointSize:20 weight:NSFontWeightSemibold];
	_iconView.image = [img imageWithSymbolConfiguration:cfg];

	if (_isOn) {
		CGFloat bgAlpha = _isPressed ? 0.38 : (_isHovered ? 0.34 : 0.25);
		CGFloat borderAlpha = _isHovered ? 0.85 : 0.60;
		self.layer.backgroundColor = [NSColor colorWithSRGBRed:0.16 green:0.56 blue:0.34 alpha:bgAlpha].CGColor;
		self.layer.borderColor = [NSColor colorWithSRGBRed:0.26 green:0.82 blue:0.50 alpha:borderAlpha].CGColor;
		_iconView.contentTintColor = [NSColor colorWithSRGBRed:0.32 green:0.90 blue:0.56 alpha:1.0];
		_stateLabel.stringValue = @"On";
		_stateLabel.textColor = [NSColor colorWithSRGBRed:0.38 green:0.92 blue:0.60 alpha:1.0];

		_keycapView.layer.backgroundColor = [NSColor colorWithSRGBRed:0.20 green:0.70 blue:0.42 alpha:0.25].CGColor;
		_keycapView.layer.borderColor = [NSColor colorWithSRGBRed:0.32 green:0.88 blue:0.55 alpha:0.45].CGColor;
		_keycapLabel.textColor = [NSColor colorWithSRGBRed:0.55 green:0.95 blue:0.70 alpha:0.95];
	} else {
		CGFloat bgAlpha = _isPressed ? 0.15 : (_isHovered ? 0.11 : 0.06);
		CGFloat borderAlpha = _isHovered ? 0.24 : 0.13;
		self.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:bgAlpha].CGColor;
		self.layer.borderColor = [NSColor colorWithWhite:1.0 alpha:borderAlpha].CGColor;
		_iconView.contentTintColor = [NSColor colorWithSRGBRed:0.96 green:0.38 blue:0.38 alpha:1.0];
		_stateLabel.stringValue = @"Muted";
		if ([_titleText isEqualToString:@"Camera"]) {
			_stateLabel.stringValue = @"Off";
		}
		_stateLabel.textColor = [NSColor colorWithWhite:0.56 alpha:1.0];

		_keycapView.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.06].CGColor;
		_keycapView.layer.borderColor = [NSColor colorWithWhite:1.0 alpha:0.15].CGColor;
		_keycapLabel.textColor = [NSColor colorWithWhite:0.55 alpha:1.0];
	}
}

- (void)mouseEntered:(NSEvent *)event {
	_isHovered = YES;
	[self updateAppearance];
}

- (void)mouseExited:(NSEvent *)event {
	_isHovered = NO;
	_isPressed = NO;
	[self updateAppearance];
}

- (void)mouseDown:(NSEvent *)event {
	[self.window makeKeyWindow];
	_isPressed = YES;
	[self updateAppearance];
}

- (void)mouseUp:(NSEvent *)event {
	if (_isPressed) {
		_isPressed = NO;
		NSPoint loc = [self convertPoint:event.locationInWindow fromView:nil];
		if (NSPointInRect(loc, self.bounds)) {
			[self toggle];
		} else {
			[self updateAppearance];
		}
	}
}

- (void)resetCursorRects {
	[self addCursorRect:self.bounds cursor:[NSCursor pointingHandCursor]];
}

@end

// --- Action Pill Button (Snooze 2m / Join Meeting) ---

@interface ActionPillButton : NSView
@property (nonatomic, copy) void (^onClick)(void);
@property (nonatomic, assign) BOOL isPrimary;
@property (nonatomic, assign) BOOL isHovered;
@property (nonatomic, assign) BOOL isPressed;
@property (nonatomic, strong) NSTextField *label;
@property (nonatomic, strong) NSImageView *iconView;
- (instancetype)initWithFrame:(NSRect)frame
                        title:(NSString *)title
                       symbol:(NSString *)symbol
                    isPrimary:(BOOL)isPrimary
                      onClick:(void (^)(void))onClick;
@end

@implementation ActionPillButton

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
	return YES;
}

- (instancetype)initWithFrame:(NSRect)frame
                        title:(NSString *)title
                       symbol:(NSString *)symbol
                    isPrimary:(BOOL)isPrimary
                      onClick:(void (^)(void))onClick {
	self = [super initWithFrame:frame];
	if (self) {
		_isPrimary = isPrimary;
		_onClick = [onClick copy];
		self.wantsLayer = YES;
		self.layer.cornerRadius = 11.0;

		NSTrackingArea *area = [[NSTrackingArea alloc] initWithRect:self.bounds
		                                                    options:NSTrackingMouseEnteredAndExited | NSTrackingActiveAlways | NSTrackingInVisibleRect
		                                                      owner:self
		                                                   userInfo:nil];
		[self addTrackingArea:area];

		CGFloat contentWidth = 0;
		if (symbol) {
			_iconView = [[NSImageView alloc] initWithFrame:NSMakeRect(0, (frame.size.height - 15) / 2.0, 15, 15)];
			NSImage *img = [NSImage imageWithSystemSymbolName:symbol accessibilityDescription:title];
			NSImageSymbolConfiguration *cfg = [NSImageSymbolConfiguration configurationWithPointSize:12.5 weight:NSFontWeightSemibold];
			_iconView.image = [img imageWithSymbolConfiguration:cfg];
			_iconView.contentTintColor = isPrimary ? [NSColor whiteColor] : [NSColor colorWithWhite:0.82 alpha:1.0];
			[self addSubview:_iconView];
			contentWidth += 21;
		}

		_label = [NSTextField labelWithString:title];
		_label.font = [NSFont systemFontOfSize:13.0 weight:isPrimary ? NSFontWeightSemibold : NSFontWeightMedium];
		_label.textColor = isPrimary ? [NSColor whiteColor] : [NSColor colorWithWhite:0.88 alpha:1.0];
		[_label sizeToFit];
		contentWidth += _label.frame.size.width;

		CGFloat startX = (frame.size.width - contentWidth) / 2.0;
		if (_iconView) {
			_iconView.frame = NSMakeRect(startX, (frame.size.height - 15) / 2.0, 15, 15);
			startX += 21;
		}
		_label.frame = NSMakeRect(startX, (frame.size.height - _label.frame.size.height) / 2.0, _label.frame.size.width, _label.frame.size.height);
		[self addSubview:_label];

		[self updateColors];
	}
	return self;
}

- (void)updateColors {
	if (_isPrimary) {
		if (_isPressed) {
			self.layer.backgroundColor = [NSColor colorWithSRGBRed:0.13 green:0.58 blue:0.32 alpha:1.0].CGColor;
		} else if (_isHovered) {
			self.layer.backgroundColor = [NSColor colorWithSRGBRed:0.20 green:0.75 blue:0.43 alpha:1.0].CGColor;
		} else {
			self.layer.backgroundColor = [NSColor colorWithSRGBRed:0.16 green:0.67 blue:0.38 alpha:1.0].CGColor;
		}
		self.layer.borderWidth = 1.0;
		self.layer.borderColor = [NSColor colorWithWhite:1.0 alpha:0.18].CGColor;
	} else {
		if (_isPressed) {
			self.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.20].CGColor;
		} else if (_isHovered) {
			self.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.14].CGColor;
		} else {
			self.layer.backgroundColor = [NSColor colorWithWhite:1.0 alpha:0.08].CGColor;
		}
		self.layer.borderWidth = 1.0;
		self.layer.borderColor = [NSColor colorWithWhite:1.0 alpha:0.14].CGColor;
	}
}

- (void)mouseEntered:(NSEvent *)event {
	_isHovered = YES;
	[self updateColors];
}

- (void)mouseExited:(NSEvent *)event {
	_isHovered = NO;
	_isPressed = NO;
	[self updateColors];
}

- (void)mouseDown:(NSEvent *)event {
	_isPressed = YES;
	[self updateColors];
}

- (void)mouseUp:(NSEvent *)event {
	if (_isPressed) {
		_isPressed = NO;
		[self updateColors];
		NSPoint loc = [self convertPoint:event.locationInWindow fromView:nil];
		if (NSPointInRect(loc, self.bounds) && _onClick) {
			_onClick();
		}
	}
}

- (void)resetCursorRects {
	[self addCursorRect:self.bounds cursor:[NSCursor pointingHandCursor]];
}

@end

// --- Floating HUD Panel ---

@interface HUDPanel : NSPanel
@property (nonatomic, copy) void (^onJoin)(void);
@property (nonatomic, copy) void (^onSnooze)(void);
@property (nonatomic, copy) void (^onDismiss)(void);
@property (nonatomic, copy) void (^onToggleMic)(void);
@property (nonatomic, copy) void (^onToggleCam)(void);
@property (nonatomic, assign) long long startUnix;
@property (nonatomic, strong) NSString *startStr;
@property (nonatomic, strong) NSTextField *badgeLabel;
@property (nonatomic, strong) NSView *dotView;
- (void)refreshElapsedBadge;
@end

@implementation HUDPanel
- (BOOL)canBecomeKeyWindow { return YES; }
- (BOOL)canBecomeMainWindow { return NO; }

- (void)refreshElapsedBadge {
	long long now = (long long)[[NSDate date] timeIntervalSince1970];
	long long elapsedSec = now - _startUnix;
	long long mins = elapsedSec / 60;

	if (elapsedSec < 60) {
		_badgeLabel.stringValue = [NSString stringWithFormat:@"STARTING NOW  •  %@", _startStr];
		_dotView.layer.backgroundColor = [NSColor colorWithSRGBRed:0.22 green:0.86 blue:0.50 alpha:1.0].CGColor;
	} else if (mins < 5) {
		_badgeLabel.stringValue = [NSString stringWithFormat:@"%lldm INTO MEETING  •  %@", mins, _startStr];
		_dotView.layer.backgroundColor = [NSColor colorWithSRGBRed:0.98 green:0.74 blue:0.18 alpha:1.0].CGColor;
	} else {
		_badgeLabel.stringValue = [NSString stringWithFormat:@"%lldm LATE  •  %@", mins, _startStr];
		_dotView.layer.backgroundColor = [NSColor colorWithSRGBRed:0.98 green:0.42 blue:0.34 alpha:1.0].CGColor;
	}
}

- (void)keyDown:(NSEvent *)event {
	unsigned short code = event.keyCode;
	NSString *chars = [event.charactersIgnoringModifiers lowercaseString];
	if (code == 36 || code == 76) { // Return / Enter
		if (_onJoin) _onJoin();
		return;
	}
	if (code == 53) { // Escape
		if (_onDismiss) _onDismiss();
		return;
	}
	if ([chars isEqualToString:@"m"]) {
		if (_onToggleMic) _onToggleMic();
		return;
	}
	if ([chars isEqualToString:@"v"]) {
		if (_onToggleCam) _onToggleCam();
		return;
	}
	if ([chars isEqualToString:@"s"]) {
		if (_onSnooze) _onSnooze();
		return;
	}
	[super keyDown:event];
}
@end

static PopupResult runNativePopup(const char *cTitle, const char *cSubtitle, const char *cStart, long long startUnix, int defaultAudio, int defaultVideo) {
	@autoreleasepool {
		NSApplication *app = [NSApplication sharedApplication];
		[app setActivationPolicy:NSApplicationActivationPolicyAccessory];

		NSString *title = [NSString stringWithUTF8String:cTitle];
		NSString *subtitle = [NSString stringWithUTF8String:cSubtitle];
		NSString *startStr = [NSString stringWithUTF8String:cStart];

		CGFloat width = 368;
		CGFloat padding = 20;
		CGFloat contentWidth = width - padding * 2;

		// Measure 1-line vs 2-line title height
		NSFont *titleFont = [NSFont systemFontOfSize:17.5 weight:NSFontWeightBold];
		NSRect titleMeasure = [title boundingRectWithSize:NSMakeSize(contentWidth - 24, 52)
		                                          options:NSStringDrawingUsesLineFragmentOrigin | NSStringDrawingUsesFontLeading
		                                       attributes:@{NSFontAttributeName: titleFont}];
		CGFloat titleHeight = ceil(titleMeasure.size.height);
		if (titleHeight < 24) titleHeight = 24;
		if (titleHeight > 46) titleHeight = 46;

		BOOL hasSubtitle = (subtitle.length > 0);
		CGFloat subtitleHeight = hasSubtitle ? 18 : 0;

		// Layout from bottom to top:
		// bottom padding: 18
		// action buttons: 40
		// gap: 14
		// toggle cards: 90
		// gap: 14
		// subtitle (optional): 18 + gap 4
		// title: titleHeight + gap 8
		// header badge: 18 + top padding 18
		CGFloat btnY = 18;
		CGFloat btnHeight = 40;
		CGFloat cardY = btnY + btnHeight + 14;
		CGFloat cardHeight = 90;
		CGFloat afterCardsY = cardY + cardHeight + 14;

		CGFloat subtitleY = afterCardsY;
		CGFloat titleY = afterCardsY + (hasSubtitle ? (subtitleHeight + 4) : 0);
		CGFloat badgeY = titleY + titleHeight + 8;
		CGFloat height = badgeY + 18 + 18;

		NSScreen *screen = [NSScreen mainScreen];
		NSRect screenFrame = screen ? screen.visibleFrame : NSMakeRect(0, 0, 1440, 900);
		// Center on the active screen
		CGFloat x = screenFrame.origin.x + (screenFrame.size.width - width) / 2.0;
		CGFloat y = screenFrame.origin.y + (screenFrame.size.height - height) / 2.0;

		HUDPanel *panel = [[HUDPanel alloc] initWithContentRect:NSMakeRect(x, y, width, height)
		                                              styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
		                                                backing:NSBackingStoreBuffered
		                                                  defer:NO];
		panel.opaque = NO;
		panel.backgroundColor = [NSColor clearColor];
		panel.hasShadow = YES;
		panel.level = NSStatusWindowLevel;
		panel.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary;
		panel.movableByWindowBackground = YES;
		panel.hidesOnDeactivate = NO;
		panel.floatingPanel = YES;
		panel.becomesKeyOnlyIfNeeded = YES;
		panel.appearance = [NSAppearance appearanceNamed:NSAppearanceNameVibrantDark];

		NSVisualEffectView *blurView = [[NSVisualEffectView alloc] initWithFrame:NSMakeRect(0, 0, width, height)];
		blurView.material = NSVisualEffectMaterialHUDWindow;
		blurView.blendingMode = NSVisualEffectBlendingModeBehindWindow;
		blurView.state = NSVisualEffectStateActive;
		blurView.wantsLayer = YES;
		blurView.layer.cornerRadius = 18.0;
		blurView.layer.masksToBounds = YES;
		blurView.layer.borderWidth = 1.0;
		blurView.layer.borderColor = [NSColor colorWithWhite:1.0 alpha:0.18].CGColor;
		panel.contentView = blurView;

		// Pulsing status dot
		NSView *dot = [[NSView alloc] initWithFrame:NSMakeRect(padding, badgeY + 4, 8, 8)];
		dot.wantsLayer = YES;
		dot.layer.cornerRadius = 4.0;
		dot.layer.backgroundColor = [NSColor colorWithSRGBRed:0.22 green:0.86 blue:0.50 alpha:1.0].CGColor;

		CABasicAnimation *pulse = [CABasicAnimation animationWithKeyPath:@"opacity"];
		pulse.fromValue = @(1.0);
		pulse.toValue = @(0.32);
		pulse.duration = 0.95;
		pulse.autoreverses = YES;
		pulse.repeatCount = HUGE_VALF;
		pulse.timingFunction = [CAMediaTimingFunction functionWithName:kCAMediaTimingFunctionEaseInEaseOut];
		[dot.layer addAnimation:pulse forKey:@"pulseOpacity"];
		[blurView addSubview:dot];

		// Live elapsed badge label
		NSTextField *badgeLabel = [NSTextField labelWithString:@""];
		badgeLabel.frame = NSMakeRect(padding + 14, badgeY, contentWidth - 42, 16);
		badgeLabel.font = [NSFont systemFontOfSize:10.5 weight:NSFontWeightBold];
		badgeLabel.textColor = [NSColor colorWithWhite:0.68 alpha:1.0];
		[blurView addSubview:badgeLabel];

		panel.startUnix = startUnix;
		panel.startStr = startStr;
		panel.badgeLabel = badgeLabel;
		panel.dotView = dot;
		[panel refreshElapsedBadge];

		// Meeting Title (up to 2 lines)
		NSTextField *titleLabel = [[NSTextField alloc] initWithFrame:NSMakeRect(padding, titleY, contentWidth - 20, titleHeight)];
		titleLabel.stringValue = title;
		titleLabel.bezeled = NO;
		titleLabel.drawsBackground = NO;
		titleLabel.editable = NO;
		titleLabel.selectable = NO;
		titleLabel.font = titleFont;
		titleLabel.textColor = [NSColor whiteColor];
		titleLabel.maximumNumberOfLines = 2;
		titleLabel.cell.wraps = YES;
		titleLabel.cell.lineBreakMode = NSLineBreakByTruncatingTail;
		[blurView addSubview:titleLabel];

		// Attendee / Context Subtitle
		if (hasSubtitle) {
			NSTextField *subLabel = [NSTextField labelWithString:subtitle];
			subLabel.frame = NSMakeRect(padding, subtitleY, contentWidth, subtitleHeight);
			subLabel.font = [NSFont systemFontOfSize:12.5 weight:NSFontWeightRegular];
			subLabel.textColor = [NSColor colorWithWhite:0.64 alpha:1.0];
			subLabel.lineBreakMode = NSLineBreakByTruncatingTail;
			[blurView addSubview:subLabel];
		}

		// Two side-by-side Toggle Cards: Audio & Video
		CGFloat cardGap = 12;
		CGFloat cardWidth = (contentWidth - cardGap) / 2.0;

		ToggleCardButton *micCard = [[ToggleCardButton alloc] initWithFrame:NSMakeRect(padding, cardY, cardWidth, cardHeight)
		                                                              title:@"Microphone"
		                                                           onSymbol:@"mic.fill"
		                                                          offSymbol:@"mic.slash.fill"
		                                                             keycap:@"M"
		                                                          initialOn:(defaultAudio != 0)];
		[blurView addSubview:micCard];

		ToggleCardButton *camCard = [[ToggleCardButton alloc] initWithFrame:NSMakeRect(padding + cardWidth + cardGap, cardY, cardWidth, cardHeight)
		                                                              title:@"Camera"
		                                                           onSymbol:@"video.fill"
		                                                          offSymbol:@"video.slash.fill"
		                                                             keycap:@"V"
		                                                          initialOn:(defaultVideo != 0)];
		[blurView addSubview:camCard];

		__block int actionResult = 0; // 0 = dismiss, 1 = join, 2 = snooze
		__block NSTimer *elapsedTimer = nil;

		void (^closeWithAction)(int) = ^(int act) {
			actionResult = act;
			if (elapsedTimer) {
				[elapsedTimer invalidate];
				elapsedTimer = nil;
			}
			[panel close];
			[app stop:nil];
			// Post a dummy event so [app run] wakes up immediately after [app stop:nil]
			NSEvent *dummy = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
			                                    location:NSMakePoint(0, 0)
			                               modifierFlags:0
			                                   timestamp:0
			                                windowNumber:0
			                                     context:nil
			                                     subtype:0
			                                       data1:0
			                                       data2:0];
			[app postEvent:dummy atStart:YES];
		};

		panel.onJoin = ^{ closeWithAction(1); };
		panel.onSnooze = ^{ closeWithAction(2); };
		panel.onDismiss = ^{ closeWithAction(0); };
		panel.onToggleMic = ^{ [micCard toggle]; };
		panel.onToggleCam = ^{ [camCard toggle]; };

		// Top-right 'X' Dismiss button
		CloseIconButton *closeBtn = [[CloseIconButton alloc] initWithFrame:NSMakeRect(width - padding - 22, height - 36, 22, 22)
		                                                           onClick:^{ closeWithAction(0); }];
		[blurView addSubview:closeBtn];

		// Bottom Buttons: "Snooze 2m" & "Join Meeting"
		CGFloat snoozeWidth = 118;
		CGFloat joinWidth = contentWidth - snoozeWidth - 10;

		ActionPillButton *snoozeBtn = [[ActionPillButton alloc] initWithFrame:NSMakeRect(padding, btnY, snoozeWidth, btnHeight)
		                                                                title:@"Snooze 2m"
		                                                               symbol:@"clock.arrow.circlepath"
		                                                            isPrimary:NO
		                                                              onClick:^{ closeWithAction(2); }];
		[blurView addSubview:snoozeBtn];

		ActionPillButton *joinBtn = [[ActionPillButton alloc] initWithFrame:NSMakeRect(padding + snoozeWidth + 10, btnY, joinWidth, btnHeight)
		                                                              title:@"Join Meeting"
		                                                             symbol:@"arrow.up.right.video.fill"
		                                                          isPrimary:YES
		                                                            onClick:^{ closeWithAction(1); }];
		[blurView addSubview:joinBtn];

		// Update live elapsed time every second
		elapsedTimer = [NSTimer scheduledTimerWithTimeInterval:1.0
		                                               repeats:YES
		                                                 block:^(NSTimer * _Nonnull timer) {
			[panel refreshElapsedBadge];
		}];

		NSSound *sound = [NSSound soundNamed:@"Glass"];
		if (sound) {
			[sound play];
		}

		// Show non-intrusively on top of active virtual desktop without stealing focus
		[panel orderFrontRegardless];
		[app run];

		PopupResult res;
		res.action = actionResult;
		res.audio_on = micCard.isOn ? 1 : 0;
		res.video_on = camCard.isOn ? 1 : 0;
		return res;
	}
}
*/
import "C"

import "unsafe"

func runPopupUI(summary, subtitle, startStr string, startUnix int64, defaults userPrefs) dialogResult {
	if summary == "" {
		summary = "Google Meet"
	}
	cTitle := C.CString(summary)
	defer C.free(unsafe.Pointer(cTitle))
	cSubtitle := C.CString(subtitle)
	defer C.free(unsafe.Pointer(cSubtitle))
	cStart := C.CString(startStr)
	defer C.free(unsafe.Pointer(cStart))

	var defAudio, defVideo C.int
	if defaults.AudioOn {
		defAudio = 1
	}
	if defaults.VideoOn {
		defVideo = 1
	}

	res := C.runNativePopup(cTitle, cSubtitle, cStart, C.longlong(startUnix), defAudio, defVideo)
	action := "dismiss"
	switch int(res.action) {
	case 1:
		action = "join"
	case 2:
		action = "snooze"
	}
	return dialogResult{
		Action:  action,
		AudioOn: res.audio_on != 0,
		VideoOn: res.video_on != 0,
	}
}
