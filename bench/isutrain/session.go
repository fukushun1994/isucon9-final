package isutrain

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"

	"github.com/chibiegg/isucon9-final/bench/internal/bencherror"
	"github.com/chibiegg/isucon9-final/bench/internal/config"
	"golang.org/x/xerrors"
)

var (
	ErrRedirect = errors.New("redirectが検出されました")
)

type Session struct {
	httpClient *http.Client
}

func NewSession() (*Session, error) {
	jar, err := cookiejar.New(&cookiejar.Options{})
	if err != nil {
		return nil, err
	}

	return &Session{
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					// ServerName: "", // FIXME: ServerName設定
					InsecureSkipVerify: true,
				},
			},
			Jar:     jar,
			Timeout: config.APITimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return bencherror.NewApplicationError(ErrRedirect, "アプリケーションへのリクエストでリダイレクトを検出しました")
			},
		},
	}, nil
}

func newSessionForInitialize() (*Session, error) {
	return &Session{
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					// ServerName: "", // FIXME: ServerName設定
					InsecureSkipVerify: true,
				},
			},
			Timeout: config.InitializeTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return bencherror.NewApplicationError(ErrRedirect, "アプリケーションへのリクエストでリダイレクトを検出しました")
			},
		},
	}, nil
}

// NOTE: GETクエリパラメータをURLにくっつける処理は、utilityなどのURLを扱う側で実装
// NOTE: Content-Type など、他のHTTPメソッドで必要なヘッダについては適宜Setする
func (sess *Session) newRequest(ctx context.Context, method, uri string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.Header.Add("User-Agent", config.UserAgent)

	return req, nil
}

func (sess *Session) do(req *http.Request) (*http.Response, error) {
	resp, err := sess.httpClient.Do(req)
	if err != nil {
		var netErr net.Error
		if xerrors.As(err, &netErr) {
			if netErr.Timeout() {
				return nil, bencherror.NewTimeoutError(err, "アプリケーションへのリクエストがタイムアウトしました")
			} else if netErr.Temporary() {
				return nil, bencherror.NewTemporaryError(err, "アプリケーションへのリクエストで一時的エラーが発生しました")
			}
		}

		return nil, bencherror.NewApplicationError(err, "アプリケーションへのリクエストが失敗しました")
	}

	return resp, nil
}
