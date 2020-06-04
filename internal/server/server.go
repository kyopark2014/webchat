package server

import (
	"sync"
	"webchat/internal/config"
	"webchat/internal/logger"
)

var log *logger.Logger

// Service is the interface for main framework
type Service interface {
	Init(conf *config.AppConfig) error
	Start() error
	//	OnTerminate() error
}

func init() {
	log = logger.NewLogger("server")
}

// BaseService is the base service
type BaseService struct {
	service Service
	wg      *sync.WaitGroup
	conf    *config.AppConfig
}

// NewBaseService defines an instance of BaseService
func NewBaseService(s Service, wg *sync.WaitGroup, conf *config.AppConfig) *BaseService {
	bs := &BaseService{service: s, wg: wg, conf: conf}

	if err := bs.service.Init(bs.conf); err != nil {
		log.E("Fail to initiate: %v", err)
	}

	bs.wg.Add(1)

	return bs
}

// Run is to start the service
func (bs *BaseService) Run() error {
	defer bs.wg.Done()

	log.D("Start Service...")

	return bs.service.Start()
}

// Stop is to finish the servce
func (bs *BaseService) Stop() error {
	defer bs.wg.Done()

	log.D("Stop Server")
	//	return bs.service.OnTerminate()
	return nil
}
