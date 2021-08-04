package main

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"github.com/chibiegg/isucon9-final/bench/internal/bencherror"
	"github.com/chibiegg/isucon9-final/bench/internal/config"
	"github.com/chibiegg/isucon9-final/bench/scenario"
	"golang.org/x/sync/semaphore"
)

var (
	ErrBenchmarkFailure = errors.New("ベンチマークに失敗しました")
)

type benchmarker struct {
	sem *semaphore.Weighted
}

func newBenchmarker() *benchmarker {
	lgr := zap.S()

	weight := int64(config.ReservationEndDate.Month())
	lgr.Infof("負荷レベル Lv:%d", weight)
	return &benchmarker{sem: semaphore.NewWeighted(weight)}
}

// ベンチ負荷の１単位. これの回転数を上げていく
func (b *benchmarker) load(ctx context.Context) error {
	defer b.sem.Release(1)

	month := int(config.ReservationEndDate.Month())

	scenario.NormalScenario(ctx)

	scenario.NormalCancelScenario(ctx)

	scenario.AttackReserveForOtherReservation(ctx)

	scenario.AttackReserveRaceCondition(ctx)

	scenario.AbnormalReserveWrongSection(ctx)

	scenario.AbnormalReserveWrongSeat(ctx)

	if month > 3 {
		scenario.NormalManyAmbigiousSearchScenario(ctx, month*3)
	}

	if month > 3 {
		scenario.NormalManyCancelScenario(ctx, month*3)
	}

	scenario.NormalVagueSearchScenario(ctx)

	if config.IsGoldenweekStarted() {
		scenario.SeasonGoldenWeekScenario(ctx, config.GoldenWeekStartDate, 5)
	}
	if config.IsGoldenweekEnded() {
		scenario.SeasonGoldenWeekScenario(ctx, config.GoldenWeekEndDate, 5)
	}

	if config.IsOlympic() {
		scenario.SeasonOlympicScenario(ctx, 5)
	}

	return nil
}

func (b *benchmarker) run(ctx context.Context) error {
	defer bencherror.BenchmarkErrs.DumpCounters()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if bencherror.BenchmarkErrs.IsFailure() {
				// 失格と分かれば、早々にベンチマークを終了
				return ErrBenchmarkFailure
			}

			if isAcquired := b.sem.TryAcquire(1); isAcquired {
				go b.load(ctx)
			}
		}
	}
}
