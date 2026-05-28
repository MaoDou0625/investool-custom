package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/axiaoxin-com/investool/webserver"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestLauncherSessionHeartbeatAndStatus(t *testing.T) {
	session := "test-launcher-session"
	resetLauncherSessionForTest(session)
	defer resetLauncherSessionForTest(session)

	viper.Set("server.mode", "release")
	viper.Set("server.host_url", "")
	viper.Set("statics.tmpl_path", "html/*")
	viper.Set("statics.url", "/statics")
	defer viper.Reset()

	app := webserver.NewGinEngine()
	Routes(app)

	statusRecorder := httptest.NewRecorder()
	statusReq, err := http.NewRequest(http.MethodGet, "/launcher/session/status?session="+session, nil)
	require.NoError(t, err)
	app.ServeHTTP(statusRecorder, statusReq)
	require.Equal(t, http.StatusOK, statusRecorder.Code)
	require.False(t, decodeLauncherSessionStatusForTest(t, statusRecorder.Body.Bytes()).Alive)

	heartbeatRecorder := httptest.NewRecorder()
	heartbeatReq, err := http.NewRequest(http.MethodPost, "/launcher/session/heartbeat?session="+session, nil)
	require.NoError(t, err)
	app.ServeHTTP(heartbeatRecorder, heartbeatReq)
	require.Equal(t, http.StatusNoContent, heartbeatRecorder.Code)

	statusRecorder = httptest.NewRecorder()
	statusReq, err = http.NewRequest(http.MethodGet, "/launcher/session/status?session="+session, nil)
	require.NoError(t, err)
	app.ServeHTTP(statusRecorder, statusReq)
	require.Equal(t, http.StatusOK, statusRecorder.Code)
	require.True(t, decodeLauncherSessionStatusForTest(t, statusRecorder.Body.Bytes()).Alive)
}

func TestLauncherSessionStatusExpires(t *testing.T) {
	session := "test-launcher-expired"
	resetLauncherSessionForTest(session)
	defer resetLauncherSessionForTest(session)

	launcherSessionMu.Lock()
	launcherSessionLastSeen[session] = time.Now().Add(-launcherSessionAliveWindow - time.Second)
	launcherSessionMu.Unlock()

	viper.Set("server.mode", "release")
	viper.Set("server.host_url", "")
	viper.Set("statics.tmpl_path", "html/*")
	viper.Set("statics.url", "/statics")
	defer viper.Reset()

	app := webserver.NewGinEngine()
	Routes(app)

	statusRecorder := httptest.NewRecorder()
	statusReq, err := http.NewRequest(http.MethodGet, "/launcher/session/status?session="+session, nil)
	require.NoError(t, err)
	app.ServeHTTP(statusRecorder, statusReq)
	require.Equal(t, http.StatusOK, statusRecorder.Code)
	require.False(t, decodeLauncherSessionStatusForTest(t, statusRecorder.Body.Bytes()).Alive)
}

type launcherSessionStatusForTest struct {
	Alive bool `json:"alive"`
}

func decodeLauncherSessionStatusForTest(t *testing.T, body []byte) launcherSessionStatusForTest {
	t.Helper()
	status := launcherSessionStatusForTest{}
	require.NoError(t, json.Unmarshal(body, &status))
	return status
}

func resetLauncherSessionForTest(session string) {
	launcherSessionMu.Lock()
	defer launcherSessionMu.Unlock()
	delete(launcherSessionLastSeen, session)
}
