package bencherror

import (
	"sync"

	"github.com/chibiegg/isucon9-final/bench/internal/config"
	"go.uber.org/zap"
)

var (
	SystemErrs     = NewBenchErrors()
	InitializeErrs = NewBenchErrors()
	PreTestErrs    = NewBenchErrors()
	BenchmarkErrs  = NewBenchErrors()
	FinalCheckErrs = NewBenchErrors()
)

type BenchErrors struct {
	mu sync.RWMutex

	Msgs         []string
	InternalMsgs []string

	criticalCnt    uint64
	applicationCnt uint64
	timeoutCnt     uint64
	temporaryCnt   uint64

	isBenchmarkFinished bool
}

func NewBenchErrors() *BenchErrors {
	return &BenchErrors{
		Msgs:         []string{},
		InternalMsgs: []string{},
	}
}

// IsError は、エラーが発生したか否かを返します
func (errs *BenchErrors) IsError() bool {
	errs.mu.RLock()
	defer errs.mu.RUnlock()

	return len(errs.Msgs) > 0
}

// IsFailure は失格したか否かを返します
func (errs *BenchErrors) IsFailure() bool {
	errs.mu.RLock()
	defer errs.mu.RUnlock()

	// if errs.criticalCnt > 0 || errs.applicationCnt >= 10 {
	if errs.criticalCnt > 0 || errs.applicationCnt >= 1000 {
		return true
	}
	return false
}

func (errs *BenchErrors) Penalty() int64 {
	errs.mu.RLock()
	defer errs.mu.RUnlock()

	lgr := zap.S()

	penalty := config.ApplicationPenaltyWeight * errs.applicationCnt
	lgr.Infof("アプリのエラーによるペナルティ: %d", penalty)

	trivialCnt := errs.timeoutCnt + errs.temporaryCnt
	if trivialCnt > config.TrivialPenaltyThreshold {
		lgr.Warn("タイムアウトや一時的なエラーが閾値を超えています")
		penalty += config.TrivialPenaltyWeight * (1 + (trivialCnt-config.TrivialPenaltyThreshold)/config.TrivialPenaltyPerCount)
		lgr.Infof("タイムアウトや一時的なエラーによるペナルティ: %d", penalty)
	}

	return int64(penalty)
}

func (errs *BenchErrors) AddError(err error) error {
	errs.mu.Lock()
	defer errs.mu.Unlock()

	if err == nil || errs.isBenchmarkFinished {
		return nil
	}

	errs.InternalMsgs = append(errs.InternalMsgs, err.Error())

	// エラーに応じたメッセージを追加し、カウンタをインクリメント
	if msg, code, ok := extractCode(err); ok {
		switch code {
		case errCritical:
			errs.Msgs = append(errs.Msgs, msg+" (critical error)")
			errs.criticalCnt++
		case errApplication:
			errs.Msgs = append(errs.Msgs, msg)
			errs.applicationCnt++
		case errTimeout:
			errs.Msgs = append(errs.Msgs, msg+" (タイムアウトしました)")
			errs.timeoutCnt++
		case errTemporary:
			errs.Msgs = append(errs.Msgs, msg+" (一時的なエラー)")
			errs.temporaryCnt++
		default:
			errs.Msgs = append(errs.Msgs, msg+" 運営に確認をお願いいたします")
			errs.criticalCnt++
		}
	}

	return err
}

func (errs *BenchErrors) DumpCounters() {
	errs.mu.Lock()
	defer errs.mu.Unlock()

	errs.isBenchmarkFinished = true

	lgr := zap.S()
	lgr.Infow("ベンチマーク完了時のエラーカウンタ",
		"critical", errs.criticalCnt,
		"application", errs.applicationCnt,
		"timeout", errs.timeoutCnt,
		"temporary", errs.temporaryCnt,
	)
}
