package server

import (
	"errors"
	"net"
	"strings"
	"time"

	"github.com/realDragonium/Ultraviolet/mc"
)

var ErrOverConnRateLimit = errors.New("too many request within rate limit time frame")

func FilterIpFromAddr(addr net.Addr) string {
	s := addr.String()
	parts := strings.Split(s, ":")
	return parts[0]
}

type ConnectionLimiter interface {
	// The process answer is empty and should be ignored when it does allow the connection to happen
	Allow(req BackendRequest) (BackendAnswer, bool)
}

func NewAbsConnLimiter(ratelimit int, cooldown time.Duration, limitStatus bool) ConnectionLimiter {
	return &absoluteConnlimiter{
		rateLimit:    ratelimit,
		rateCooldown: cooldown,
		limitStatus:  limitStatus,
	}
}

type absoluteConnlimiter struct {
	rateCounter   int
	rateStartTime time.Time
	rateLimit     int
	rateCooldown  time.Duration
	limitStatus   bool
}

func (r *absoluteConnlimiter) Allow(req BackendRequest) (BackendAnswer, bool) {
	if time.Since(r.rateStartTime) >= r.rateCooldown {
		r.rateCounter = 0
		r.rateStartTime = time.Now()
	}
	if !r.limitStatus {
		return BackendAnswer{}, true
	}
	if r.rateCounter < r.rateLimit {
		r.rateCounter++
		return BackendAnswer{}, true
	}
	return NewCloseAnswer(), false
}

type AlwaysAllowConnection struct{}

func (limiter AlwaysAllowConnection) Allow(req BackendRequest) (BackendAnswer, bool) {
	return BackendAnswer{}, true
}

func NewBotFilterConnLimiter(ratelimit int, cooldown, clearTime time.Duration, disconnPk mc.Packet) ConnectionLimiter {
	return &botFilterConnLimiter{
		rateLimit:     ratelimit,
		rateCooldown:  cooldown,
		disconnPacket: disconnPk,
		listClearTime: clearTime,

		namesList: make(map[string]string),
		blackList: make(map[string]time.Time),
	}
}

type botFilterConnLimiter struct {
	rateCounter   int
	rateStartTime time.Time
	rateLimit     int
	rateCooldown  time.Duration
	disconnPacket mc.Packet
	listClearTime time.Duration

	blackList map[string]time.Time
	namesList map[string]string
}

func (limiter *botFilterConnLimiter) Allow(req BackendRequest) (BackendAnswer, bool) {
	if req.Type == mc.Status {
		return BackendAnswer{}, true
	}
	if time.Since(limiter.rateStartTime) >= limiter.rateCooldown {
		limiter.rateCounter = 0
		limiter.rateStartTime = time.Now()
	}
	limiter.rateCounter++
	ip := FilterIpFromAddr(req.Addr)
	blockTime, ok := limiter.blackList[ip]
	if time.Since(blockTime) >= limiter.listClearTime {
		delete(limiter.blackList, ip)
	} else if ok {
		return NewCloseAnswer(), false
	}

	// TODO: if connections are above rate limit, the next cooldown is probably still is
	//  Find something to improve it
	if limiter.rateCounter > limiter.rateLimit {
		username, ok := limiter.namesList[ip]
		if !ok {
			limiter.namesList[ip] = req.Username
			return NewDisconnectAnswer(limiter.disconnPacket), false
		}
		if username != req.Username {
			limiter.blackList[ip] = time.Now()
			return NewCloseAnswer(), false
		}
	}
	return BackendAnswer{}, true
}