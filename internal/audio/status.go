package audio

import "time"

// SilenceThreshold is about -50 dBFS.
const SilenceThreshold float32 = 0.0031622776

// LabelHysteresis is how long a side must stay changed before the label follows.
const LabelHysteresis = 150 * time.Millisecond

// SideHold tracks one meter side so the status label does not flicker.
type SideHold struct {
	active  bool
	pending *bool
	since   time.Time
}

// Update returns whether the side should currently be treated as audible.
func (s *SideHold) Update(now time.Time, level float32) bool {
	above := level >= SilenceThreshold
	if above == s.active {
		s.pending = nil
		return s.active
	}
	if s.pending == nil || *s.pending != above {
		v := above
		s.pending = &v
		s.since = now
		return s.active
	}
	if now.Sub(s.since) >= LabelHysteresis {
		s.active = above
		s.pending = nil
	}
	return s.active
}

// Label describes a stereo pair after hysteresis has been applied.
func Label(left, right bool) string {
	switch {
	case left && right:
		return "Both"
	case left:
		return "Left only"
	case right:
		return "Right only"
	default:
		return "Silent"
	}
}
