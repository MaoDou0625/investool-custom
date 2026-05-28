package routes

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const launcherSessionAliveWindow = 12 * time.Second

var (
	launcherSessionMu       sync.Mutex
	launcherSessionLastSeen = map[string]time.Time{}
)

// LauncherSessionHeartbeat records that a launcher-owned browser page is still open.
func LauncherSessionHeartbeat(c *gin.Context) {
	session := c.Query("session")
	if session == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session"})
		return
	}

	launcherSessionMu.Lock()
	launcherSessionLastSeen[session] = time.Now()
	launcherSessionMu.Unlock()

	c.Status(http.StatusNoContent)
}

// LauncherSessionStatus returns whether a launcher-owned browser page is still active.
func LauncherSessionStatus(c *gin.Context) {
	session := c.Query("session")
	if session == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing session"})
		return
	}

	lastSeen := launcherSessionLastSeenAt(session)
	alive := false
	ageSeconds := -1
	if !lastSeen.IsZero() {
		age := time.Since(lastSeen)
		ageSeconds = int(age.Seconds())
		alive = age <= launcherSessionAliveWindow
	}

	c.JSON(http.StatusOK, gin.H{
		"alive":       alive,
		"age_seconds": ageSeconds,
		"last_seen":   formatFund4433RecommendationTime(lastSeen),
	})
}

func launcherSessionLastSeenAt(session string) time.Time {
	launcherSessionMu.Lock()
	defer launcherSessionMu.Unlock()
	return launcherSessionLastSeen[session]
}
