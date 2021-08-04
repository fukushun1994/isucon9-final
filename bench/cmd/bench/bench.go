package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chibiegg/isucon9-final/bench/assets"
	"github.com/chibiegg/isucon9-final/bench/internal/bencherror"
	"github.com/chibiegg/isucon9-final/bench/internal/config"
	"github.com/chibiegg/isucon9-final/bench/internal/endpoint"
	"github.com/chibiegg/isucon9-final/bench/internal/logger"
	"github.com/chibiegg/isucon9-final/bench/isutrain"
	"github.com/chibiegg/isucon9-final/bench/mock"
	"github.com/chibiegg/isucon9-final/bench/payment"
	"github.com/chibiegg/isucon9-final/bench/scenario"
	"github.com/jarcoal/httpmock"
	"github.com/urfave/cli"
	"go.uber.org/zap"
)

var (
	assetDir string
)

type BenchResult struct {
	Pass          bool     `json:"pass"`
	Score         int64    `json:"score"`
	Messages      []string `json:"messages"`
	AvailableDays int      `json:"available_days"`
	Language      string   `json:"language"`
}

// UniqueMsgs は重複除去したメッセージ配列を返します
func uniqueMsgs(msgs []string) (uniqMsgs []string) {
	dedup := map[string]struct{}{}
	for _, msg := range msgs {
		if _, ok := dedup[msg]; ok {
			continue
		}
		dedup[msg] = struct{}{}
		uniqMsgs = append(uniqMsgs, msg)
	}
	return
}

func dumpFailedResult(messages []string) {
	lgr := zap.S()

	b, err := json.Marshal(&BenchResult{
		Pass:          false,
		Score:         0,
		Messages:      messages,
		AvailableDays: config.AvailableDays,
		Language:      config.Language,
	})
	if err != nil {
		lgr.Warnf("FAILEDな結果を書き出す際にエラーが発生. messagesが失われました: messages=%+v err=%+v", messages, err)
		fmt.Println(fmt.Sprintf(`{"pass": false, "score": 0, "messages": ["%s"]}`, string(b)))
	}

	fmt.Println(string(b))
}

var run = cli.Command{
	Name:  "run",
	Usage: "ベンチマーク実行",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:        "debug",
			Destination: &config.Debug,
			EnvVar:      "BENCH_DEBUG",
		},
		cli.StringFlag{
			Name:        "payment",
			Value:       "http://localhost:5000",
			Destination: &config.PaymentBaseURL,
			EnvVar:      "BENCH_PAYMENT_URL",
		},
		cli.StringFlag{
			Name:        "target",
			Value:       "http://localhost",
			Destination: &config.TargetBaseURL,
			EnvVar:      "BENCH_TARGET_URL",
		},
		cli.StringFlag{
			Name:        "assetdir",
			Value:       "assets/testdata",
			Destination: &assetDir,
			EnvVar:      "BENCH_ASSETDIR",
		},
		cli.StringFlag{
			Name:        "webhookurl",
			Destination: &config.SlackWebhookURL,
			EnvVar:      "BENCH_SLACK_WEBHOOK_URL",
		},
	},
	Action: func(cliCtx *cli.Context) error {
		ctx := context.Background()

		lgr, err := logger.InitZapLogger()
		if err != nil {
			dumpFailedResult([]string{})
			return cli.NewExitError(err, 1)
		}

		lgr.Info("===== Prepare benchmarker =====")

		assets, err := assets.Load(assetDir)
		if err != nil {
			lgr.Warn("静的ファイルをローカルから読み出せませんでした: %+v", err)
			dumpFailedResult([]string{})
			return cli.NewExitError(err, 1)
		}

		initClient, err := isutrain.NewClientForInitialize()
		if err != nil {
			lgr.Warn("isutrainクライアント生成に失敗しました: %+v", err)
			dumpFailedResult([]string{})
			return cli.NewExitError(err, 1)
		}

		testClient, err := isutrain.NewClient()
		if err != nil {
			lgr.Warn("pretestクライアント生成に失敗しました: %+v", err)
			dumpFailedResult([]string{})
			return cli.NewExitError(err, 1)
		}

		paymentClient, err := payment.NewClient()
		if err != nil {
			lgr.Warn("課金クライアント生成に失敗しました: %+v", err)
			dumpFailedResult([]string{})
			return cli.NewExitError(err, 1)
		}

		if config.Debug {
			lgr.Warn("!!!!! Debug enabled !!!!!")
			httpmock.Activate()
			defer httpmock.DeactivateAndReset()

			_, err = mock.Register()
			if err != nil {
				dumpFailedResult([]string{err.Error()})
				return cli.NewExitError(err, 1)
			}
			initClient.ReplaceMockTransport()
			testClient.ReplaceMockTransport()
		}

		// initialize
		lgr.Info("===== Initialize payment =====")
		if err := paymentClient.Initialize(); err != nil {
			lgr.Warnf("課金APIへの /initialize でエラーが発生: %s", err.Error())
			dumpFailedResult(bencherror.InitializeErrs.Msgs)
			return nil
		}
		lgr.Info("===== Initialize webapp =====")
		initClient.Initialize(ctx)
		if bencherror.InitializeErrs.IsError() {
			lgr.Warnf("webappへの /initialize でエラーが発生: %+v", bencherror.InitializeErrs.InternalMsgs)
			dumpFailedResult(bencherror.InitializeErrs.Msgs)
			return nil
		}

		// pretest (まず、正しく動作できているかチェック. エラーが見つかったら、採点しようがないのでFAILにする)
		lgr.Info("===== Pretest webapp =====")
		scenario.Pretest(ctx, testClient, paymentClient, assets)
		if bencherror.PreTestErrs.IsError() {
			lgr.Warnf("webappへの pretest でエラーが発生: %+v", bencherror.PreTestErrs.InternalMsgs)
			dumpFailedResult(bencherror.PreTestErrs.Msgs)
			return nil
		}

		// bench (ISUCOIN売り上げ計上と、減点カウントを行う)
		lgr.Info("===== Benchmark webapp =====")
		benchCtx, cancel := context.WithTimeout(context.Background(), config.BenchmarkTimeout)
		defer cancel()

		bgCtx, bgCancel := context.WithCancel(ctx)
		bgtester, err := newBgTester()
		if err != nil {
			dumpFailedResult(uniqueMsgs(bencherror.BenchmarkErrs.Msgs))
			return nil
		}
		go bgtester.run(bgCtx)

		benchmarker := newBenchmarker()
		if err := benchmarker.run(benchCtx); err != nil {
			lgr.Warnf("ベンチマークにてエラーが発生しました: %+v", err)
		}
		bgCancel()
		if bencherror.BenchmarkErrs.IsFailure() {
			dumpFailedResult(uniqueMsgs(bencherror.BenchmarkErrs.Msgs))
			return nil
		}

		lgr.Info("===== Final check =====")
		// NOTE: bulkリクエストの遅延処理考慮で、５秒待つ
		time.Sleep(5 * time.Second)
		scenario.FinalCheck(ctx, testClient, paymentClient)
		if bencherror.FinalCheckErrs.IsFailure() {
			lgr.Warnf("webappへのfinalcheckで失格判定: %+v", bencherror.FinalCheckErrs.InternalMsgs)
			msgs := append(uniqueMsgs(bencherror.BenchmarkErrs.Msgs), bencherror.FinalCheckErrs.Msgs...)
			dumpFailedResult(msgs)
			return nil
		}

		lgr.Info("===== System errors =====")
		if bencherror.SystemErrs.IsError() {
			for _, errMsg := range bencherror.SystemErrs.InternalMsgs {
				lgr.Warn(errMsg)
			}
		}

		// posttest (ベンチ後の整合性チェックにより、減点カウントを行う)
		lgr.Info("===== Calculate final score =====")

		scoreMsgs := []string{
			fmt.Sprintf("エンドポイント成功回数: %d", endpoint.CalcFinalEndpointCount()),
		}

		score := endpoint.CalcFinalScore()
		lgr.Infof("Final score: %d", score)
		scoreMsgs = append(scoreMsgs, fmt.Sprintf("スコア: %d", score))

		// エラーカウントから、スコアを減点
		score -= bencherror.BenchmarkErrs.Penalty()
		lgr.Infof("Final score (with penalty): %d", score)
		scoreMsgs = append(scoreMsgs, fmt.Sprintf("ペナルティ: %d", bencherror.BenchmarkErrs.Penalty()))

		// 最終結果をstdoutへ書き出す
		resultBytes, err := json.Marshal(&BenchResult{
			Pass:          true,
			Score:         score,
			Messages:      append(uniqueMsgs(bencherror.BenchmarkErrs.Msgs), scoreMsgs...),
			AvailableDays: config.AvailableDays,
			Language:      config.Language,
		})
		if err != nil {
			lgr.Warn("ベンチマーク結果のMarshalに失敗しました: %+v", err)
			return cli.NewExitError(err, 1)
		}
		fmt.Println(string(resultBytes))

		return nil
	},
}
