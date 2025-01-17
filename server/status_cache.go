package server

import (
	"errors"
	"time"

	"github.com/realDragonium/Ultraviolet/mc"
)

type StatusCache interface {
	Status() (BackendAnswer, error)
}

func NewStatusCache(protocol int, cooldown time.Duration, connCreator ConnectionCreator) StatusCache {
	handshake := mc.ServerBoundHandshake{
		ProtocolVersion: protocol,
		ServerAddress:   "Ultraviolet",
		ServerPort:      25565,
		NextState:       1,
	}

	return &statusCache{
		connCreator: connCreator,
		cooldown:    cooldown,
		handshake:   handshake,
	}
}

type statusCache struct {
	connCreator ConnectionCreator

	status    BackendAnswer
	cooldown  time.Duration
	cacheTime time.Time
	handshake mc.ServerBoundHandshake
}

var ErrStatusPing = errors.New("something went wrong while pinging")

func (cache *statusCache) Status() (BackendAnswer, error) {
	if time.Since(cache.cacheTime) < cache.cooldown {
		return cache.status, nil
	}
	answer, err := cache.newStatus()
	if err != nil && !errors.Is(err, ErrStatusPing) {
		return cache.status, err
	}
	cache.cacheTime = time.Now()
	cache.status = answer
	return cache.status, nil
}

func (cache *statusCache) newStatus() (BackendAnswer, error) {
	var answer BackendAnswer
	connFunc := cache.connCreator.Conn()
	conn, err := connFunc()
	if err != nil {
		return answer, err
	}
	mcConn := mc.NewMcConn(conn)
	if err := mcConn.WriteMcPacket(cache.handshake); err != nil {
		return answer, err
	}
	if err := mcConn.WritePacket(mc.ServerBoundRequest{}.Marshal()); err != nil {
		return answer, err
	}
	pk, err := mcConn.ReadPacket()
	if err != nil {
		return answer, err
	}
	conn.Close()
	return NewStatusAnswer(pk), nil
}
