package audio

import (
	"math"
	"testing"
	"time"
)

func TestGroupPeaks71(t *testing.T) {
	mask := SpeakerFrontLeft | SpeakerFrontRight | SpeakerCentre | SpeakerLFE |
		SpeakerRearLeft | SpeakerRearRight | SpeakerSideLeft | SpeakerSideRight
	peaks := []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}
	got := GroupPeaks(mask, peaks)
	if len(got) != 4 {
		t.Fatalf("pairs: got %d", len(got))
	}
	if got[0].Pair.ID != "front" || got[0].Left != 0.1 || got[0].Right != 0.2 {
		t.Fatalf("front: %+v", got[0])
	}
	if got[1].Pair.ID != "centre-lfe" || got[1].Left != 0.3 || got[1].Right != 0.4 {
		t.Fatalf("centre: %+v", got[1])
	}
	if got[2].Left != 0.5 || got[3].Right != 0.8 {
		t.Fatalf("rear/side: %+v %+v", got[2], got[3])
	}
}

func TestWithSessionPeakFillsSilentFront(t *testing.T) {
	got := WithSessionPeak(nil, 0.4)
	if len(got) != 1 || got[0].Left != 0.4 || got[0].Right != 0.4 {
		t.Fatalf("%+v", got)
	}
	stereo := GroupPeaks(SpeakerFrontLeft|SpeakerFrontRight, []float32{0.2, 0.9})
	kept := WithSessionPeak(stereo, 0.1)
	if kept[0].Left != 0.2 || kept[0].Right != 0.9 {
		t.Fatalf("kept stereo: %+v", kept[0])
	}
}

func TestGroupPeaksStereo(t *testing.T) {
	got := GroupPeaks(SpeakerFrontLeft|SpeakerFrontRight, []float32{0.4, 0.0})
	if len(got) != 1 || got[0].Left != 0.4 || got[0].Right != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestChannelIndexSideLeft(t *testing.T) {
	mask := SpeakerFrontLeft | SpeakerFrontRight | SpeakerSideLeft | SpeakerSideRight
	if ChannelIndex(mask, 4, "side", "left") != 2 {
		t.Fatal(ChannelIndex(mask, 4, "side", "left"))
	}
}

func TestThreshold(t *testing.T) {
	want := math.Pow(10, -50.0/20.0)
	if math.Abs(float64(SilenceThreshold)-want) > 1e-7 {
		t.Fatalf("threshold %v want %v", SilenceThreshold, want)
	}
}

func TestHysteresis(t *testing.T) {
	var s SideHold
	start := time.Unix(0, 0)
	if s.Update(start, 1) {
		t.Fatal("should stay silent until the hold elapses")
	}
	if !s.Update(start.Add(LabelHysteresis), 1) {
		t.Fatal("should become audible")
	}
	if !s.Update(start.Add(LabelHysteresis+time.Millisecond), 0) {
		t.Fatal("should stay audible during hysteresis")
	}
	if s.Update(start.Add(2*LabelHysteresis+time.Millisecond), 0) {
		t.Fatal("should return to silent")
	}
	if Label(true, false) != "Left only" || Label(false, true) != "Right only" || Label(true, true) != "Both" || Label(false, false) != "Silent" {
		t.Fatal("labels")
	}
}
