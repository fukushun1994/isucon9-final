package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/chibiegg/isucon9-final/bench/internal/alert"
	"github.com/chibiegg/isucon9-final/bench/internal/config"
	"github.com/eapache/go-resiliency/retrier"
	"github.com/urfave/cli"
)

var (
	targetPort                    int
	portalBaseURI, paymentBaseURI string
	benchmarkerPath               string
	assetDir                      string

	dequeueInterval int

	retryLimit, retryInterval int

	messageLimit int
)

var (
	errJobNotFound      = errors.New("ジョブが見つかりませんでした")
	errReportFailed     = errors.New("ベンチ結果報告に失敗しました")
	errAllowIPsNotFound = errors.New("許可すべきIPが見つかりませんでした")
)

const (
	MsgTimeout = "ベンチマーク処理がタイムアウトしました"
	MsgFail    = "運営に連絡してください"
)

const (
	StatusSuccess = "done"
	StatusFailed  = "aborted"
	StatusTimeout = "aborted"
)

func joinN(messages []string, n int) string {
	if len(messages) > n {
		return strings.Join(messages[:n], ",\n")
	}
	return strings.Join(messages, ",\n")
}

// ベンチマーカー実行ファイルを実行
func execBench(ctx context.Context, job *Job) (*Result, error) {
	// ターゲットサーバを取得
	targetServer, err := getTargetServer(job)
	if err != nil {
		alert.NotifyWorkerErr(job.ID, job.Team.ID, job.Team.Name, err, "", "", "ターゲットサーバの取得に失敗しました: job=%d", job.ID)
		log.Printf("failed to get target server: %s", err.Error())
		return nil, err
	}

	targetURI := fmt.Sprintf("https://%s:%d", targetServer.GlobalIP, targetPort)

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, benchmarkerPath, []string{
		"run",
		"--payment=" + paymentBaseURI,
		"--target=" + targetURI,
		"--assetdir=" + assetDir,
		"--webhookurl=" + config.SlackWebhookURL,
	}...)
	log.Printf("exec_path=%s", cmd.Path)
	for _, arg := range cmd.Args {
		log.Printf("\t- args=%s\n", arg)
	}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	status := StatusSuccess

	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Run()
	}()

	select {
	case err := <-errCh:
		if err != nil {
			alert.NotifyWorkerErr(job.ID, job.Team.ID, job.Team.Name, err, string(stdout.Bytes()), string(stderr.Bytes()), "ベンチの実行エラーが発生 (StatusFailed)")
			status = StatusFailed
		}
	case <-ctx.Done():
		alert.NotifyWorkerErr(job.ID, job.Team.ID, job.Team.Name, err, string(stdout.Bytes()), string(stderr.Bytes()), "ベンチのタイムアウトエラーが発生 (StatusTimeout)")
		status = StatusTimeout
	}

	log.Println(stdout.Bytes())

	// ベンチ結果をUnmarshal
	log.Printf("bench result = %s\n", string(stdout.Bytes()))
	var result *BenchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		alert.NotifyWorkerErr(job.ID, job.Team.ID, job.Team.Name, err, string(stdout.Bytes()), string(stderr.Bytes()), "ベンチ結果のUnmarshalに失敗しました")
		log.Println(string(stdout.Bytes()))
		return &Result{
			ID:       job.ID,
			Stdout:   string(stdout.Bytes()),
			Stderr:   string(stderr.Bytes()),
			Reason:   "ベンチ結果の取得に失敗しました. 運営に報告してください",
			IsPassed: false,
			Score:    -1,
			Status:   StatusFailed,
		}, nil
	}

	return &Result{
		ID:       job.ID,
		Stdout:   string(stdout.Bytes()),
		Stderr:   string(stderr.Bytes()),
		Reason:   joinN(result.Messages, messageLimit),
		IsPassed: result.Pass,
		Score:    result.Score,
		Status:   status,
	}, nil
}

var run = cli.Command{
	Name:  "run",
	Usage: "ベンチマークワーカー実行",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:        "portal",
			Value:       "http://localhost:8000",
			Destination: &portalBaseURI,
			EnvVar:      "BENCHWORKER_PORTAL_URL",
		},
		cli.StringFlag{
			Name:        "payment",
			Value:       "http://localhost:5000",
			Destination: &paymentBaseURI,
			EnvVar:      "BENCHWORKER_PAYMENT_URL",
		},
		cli.IntFlag{
			Name:        "target-port",
			Value:       443,
			Destination: &targetPort,
			EnvVar:      "BENCHWORKER_TARGET_PORT",
		},
		cli.StringFlag{
			Name:        "assetdir",
			Value:       "/home/isucon/isutrain/assets",
			Destination: &assetDir,
			EnvVar:      "BENCHWORKER_ASSETDIR",
		},
		cli.StringFlag{
			Name:        "benchmarker",
			Value:       "/home/isucon/isutrain/bin/benchmarker",
			Destination: &benchmarkerPath,
			EnvVar:      "BENCHWORKER_BENCHMARKER_BINPATH",
		},
		cli.IntFlag{
			Name:        "retrylimit",
			Value:       10,
			Destination: &retryLimit,
			EnvVar:      "BENCHWORKER_RETRY_LIMIT",
		},
		cli.IntFlag{
			Name:        "retryinterval",
			Value:       2,
			Destination: &retryInterval,
			EnvVar:      "BENCHWORKER_RETRY_INTERVAL",
		},
		cli.IntFlag{
			Name:        "message-limit",
			Value:       10,
			Destination: &messageLimit,
			EnvVar:      "BENCHWORKER_MESSAGE_LIMIT",
		},
		cli.StringFlag{
			Name:        "webhookurl",
			Destination: &config.SlackWebhookURL,
			EnvVar:      "BENCHWORKER_SLACK_WEBHOOK_URL",
		},
	},
	Action: func(cliCtx *cli.Context) error {
		ctx := context.Background()
		var reportWg sync.WaitGroup

		sigCh := make(chan os.Signal, 1)
		defer close(sigCh)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
	loop:
		for {
			select {
			case <-sigCh:
				break loop
			case <-ticker.C:
				job, err := dequeue(ctx)
				if err != nil {
					// dequeueが失敗しても終了しない
					continue
				}
				log.Printf("dequeue job id=%d team_id=%d target_server=%+v", job.ID, job.Team.ID, job.Team.Servers)

				reportRetrier := retrier.New(retrier.ConstantBackoff(retryLimit, time.Duration(retryInterval)*time.Second), nil)

				log.Println("===== Execute benchmarker =====")
				result, err := execBench(ctx, job)
				if err != nil {
					log.Printf("bench failed: %s\n", err.Error())
					// FIXME: ベンチ失敗した時のaction
					reportErr := reportRetrier.RunCtx(ctx, func(ctx context.Context) error {
						return report(ctx, job.ID, &Result{
							ID:       job.ID,
							Status:   StatusFailed,
							IsPassed: false,
							Reason:   err.Error(),
						})
					})
					log.Println(reportErr)
					break
				}

				log.Println("===== Report result =====")
				// ポータルに結果を報告
				reportWg.Add(1)
				go func() {
					defer reportWg.Done()
					err = reportRetrier.RunCtx(ctx, func(ctx context.Context) error {
						return report(ctx, job.ID, result)
					})
					if err != nil {
						alert.NotifyWorkerErr(job.ID, job.Team.ID, job.Team.Name, err, result.Stdout, result.Stderr, "リトライしましたが、ポータルへの報告が失敗しました")
						log.Printf("report failed: %s\n", err.Error())
					}
				}()
			}
		}

		reportWg.Wait()

		return nil
	},
}
