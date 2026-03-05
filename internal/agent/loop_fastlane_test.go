package agent

import "testing"

func TestShouldUseToolFreeMode_DefaultTelegramOnly(t *testing.T) {
	t.Setenv("GOCLAW_FAST_LANE_ALL_CHANNELS", "false")

	if !shouldUseToolFreeMode(RunRequest{
		Channel:  "telegram",
		PeerKind: "direct",
		Message:  "привет",
	}) {
		t.Fatalf("expected telegram direct short message to use tool-free mode")
	}

	if shouldUseToolFreeMode(RunRequest{
		Channel:  "ws",
		PeerKind: "direct",
		Message:  "привет",
	}) {
		t.Fatalf("expected ws direct message to keep tools when fast lane is disabled")
	}
}

func TestShouldUseToolFreeMode_AllChannelsFlag(t *testing.T) {
	t.Setenv("GOCLAW_FAST_LANE_ALL_CHANNELS", "true")

	if !shouldUseToolFreeMode(RunRequest{
		Channel:  "ws",
		PeerKind: "direct",
		Message:  "краткий ответ",
	}) {
		t.Fatalf("expected direct ws message to use tool-free mode with fast lane flag")
	}

	if shouldUseToolFreeMode(RunRequest{
		Channel:  "ws",
		PeerKind: "group",
		Message:  "краткий ответ",
	}) {
		t.Fatalf("expected group message to keep tools enabled")
	}

	if shouldUseToolFreeMode(RunRequest{
		Channel:  "ws",
		PeerKind: "direct",
		Message:  "прочитай файл и исправь код",
	}) {
		t.Fatalf("expected tool-oriented message to keep tools enabled")
	}
}

func TestChooseMaxIterations_Dynamic(t *testing.T) {
	t.Setenv("GOCLAW_DYNAMIC_MAX_ITER", "true")

	got := chooseMaxIterations(20, RunRequest{
		Message: "привет",
	}, false)
	if got != 2 {
		t.Fatalf("expected short chat to use 2 iterations, got %d", got)
	}

	got = chooseMaxIterations(20, RunRequest{
		Message: "прочитай файл app.go и исправь ошибку",
	}, false)
	if got != 3 {
		t.Fatalf("expected tool-oriented short task to use 3 iterations, got %d", got)
	}

	got = chooseMaxIterations(20, RunRequest{
		Message: "привет",
	}, true)
	if got != 1 {
		t.Fatalf("expected tool-free path to force 1 iteration, got %d", got)
	}
}

func TestChooseMaxIterations_RespectsRequestOverride(t *testing.T) {
	t.Setenv("GOCLAW_DYNAMIC_MAX_ITER", "true")
	got := chooseMaxIterations(20, RunRequest{
		Message:       "прочитай файл app.go и исправь ошибку",
		MaxIterations: 2,
	}, false)
	if got != 2 {
		t.Fatalf("expected explicit request max iterations to win, got %d", got)
	}
}

func TestChooseMaxIterations_CanBeDisabled(t *testing.T) {
	t.Setenv("GOCLAW_DYNAMIC_MAX_ITER", "false")
	got := chooseMaxIterations(20, RunRequest{
		Message: "привет",
	}, false)
	if got != 20 {
		t.Fatalf("expected default max iterations when dynamic is disabled, got %d", got)
	}
}
