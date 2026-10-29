package audio

// Speaker positions follow the WAVEFORMATEXTENSIBLE channel mask.
const (
	SpeakerFrontLeft  uint32 = 0x1
	SpeakerFrontRight uint32 = 0x2
	SpeakerCentre     uint32 = 0x4
	SpeakerLFE        uint32 = 0x8
	SpeakerRearLeft   uint32 = 0x10
	SpeakerRearRight  uint32 = 0x20
	SpeakerSideLeft   uint32 = 0x200
	SpeakerSideRight  uint32 = 0x400
)

type pairDef struct {
	id, label   string
	left, right uint32
}

var pairOrder = []pairDef{
	{"front", "Front", SpeakerFrontLeft, SpeakerFrontRight},
	{"centre-lfe", "Centre / LFE", SpeakerCentre, SpeakerLFE},
	{"rear", "Rear", SpeakerRearLeft, SpeakerRearRight},
	{"side", "Side", SpeakerSideLeft, SpeakerSideRight},
}

// WithSessionPeak keeps a measured stereo layout, and uses the session's
// overall peak for the front pair when every channel peak is silent. HDMI
// outputs often report that overall peak and nothing per channel.
func WithSessionPeak(levels []PairLevels, mono float32) []PairLevels {
	const audible = float32(0.005)
	for _, lv := range levels {
		if lv.Left >= audible || lv.Right >= audible {
			return levels
		}
	}
	if mono < audible {
		if len(levels) == 0 {
			return []PairLevels{{Pair: Pair{ID: "front", Label: "Front"}}}
		}
		return levels
	}
	if len(levels) == 0 {
		return []PairLevels{{
			Pair:  Pair{ID: "front", Label: "Front"},
			Left:  mono,
			Right: mono,
		}}
	}
	out := append([]PairLevels(nil), levels...)
	for i := range out {
		if out[i].Pair.ID == "front" {
			out[i].Left = mono
			out[i].Right = mono
			return out
		}
	}
	out[0].Left = mono
	out[0].Right = mono
	return out
}

// GroupPeaks maps WASAPI peak values onto stereo pairs.
// Channels are ordered by ascending mask bit, which is the mix-format order.
// A zero mask is treated as sequential stereo pairs in the same order.
func GroupPeaks(mask uint32, peaks []float32) []PairLevels {
	index := map[uint32]int{}
	if mask == 0 {
		seq := []uint32{
			SpeakerFrontLeft, SpeakerFrontRight,
			SpeakerCentre, SpeakerLFE,
			SpeakerRearLeft, SpeakerRearRight,
			SpeakerSideLeft, SpeakerSideRight,
		}
		for i := range peaks {
			if i < len(seq) {
				index[seq[i]] = i
			}
		}
		if len(peaks) == 1 {
			p := peaks[0]
			return []PairLevels{{
				Pair:  Pair{ID: "front", Label: "Front"},
				Left:  p,
				Right: p,
			}}
		}
	} else {
		n := 0
		for bit := uint32(1); bit != 0 && n < len(peaks); bit <<= 1 {
			if mask&bit != 0 {
				index[bit] = n
				n++
			}
		}
	}

	out := make([]PairLevels, 0, 4)
	for _, def := range pairOrder {
		li, lok := index[def.left]
		ri, rok := index[def.right]
		if !lok && !rok {
			continue
		}
		var lv PairLevels
		lv.Pair = Pair{ID: def.id, Label: def.label}
		if lok {
			lv.Left = peaks[li]
		}
		if rok {
			lv.Right = peaks[ri]
		}
		out = append(out, lv)
	}
	return out
}

// ChannelIndex returns the peak-array index for one side of a pair, or -1.
// side is "left" or "right".
func ChannelIndex(mask uint32, channels int, pairID, side string) int {
	if channels < 1 {
		return -1
	}
	index := speakerIndex(mask, channels)
	left, right := PairBits(pairID)
	bit := left
	if side == "right" {
		bit = right
	}
	if i, ok := index[bit]; ok {
		return i
	}
	return -1
}

func speakerIndex(mask uint32, channels int) map[uint32]int {
	index := map[uint32]int{}
	if mask == 0 {
		seq := []uint32{
			SpeakerFrontLeft, SpeakerFrontRight,
			SpeakerCentre, SpeakerLFE,
			SpeakerRearLeft, SpeakerRearRight,
			SpeakerSideLeft, SpeakerSideRight,
		}
		for i := 0; i < channels && i < len(seq); i++ {
			index[seq[i]] = i
		}
		return index
	}
	n := 0
	for bit := uint32(1); bit != 0 && n < channels; bit <<= 1 {
		if mask&bit != 0 {
			index[bit] = n
			n++
		}
	}
	return index
}

// Sides returns the speaker bits to fill for a test tone.
func Sides(pairID, side string) (left, right bool) {
	switch side {
	case "left":
		return true, false
	case "right":
		return false, true
	default:
		return true, true
	}
}

// PairBits returns the speaker bits for a pair.
func PairBits(pairID string) (left, right uint32) {
	for _, def := range pairOrder {
		if def.id == pairID {
			return def.left, def.right
		}
	}
	return SpeakerFrontLeft, SpeakerFrontRight
}
