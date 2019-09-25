package etcdc3

import (
	"context"
	"crypto/tls"
	"errors"
	"github.com/coreos/etcd/pkg/logutil"
	"go.uber.org/zap"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	c3 "github.com/coreos/etcd/clientv3"
	"github.com/op/go-logging"
	"github.com/zzxun/netools/util"
)

func init() {
	logger = logging.MustGetLogger("etcdc3")
}

// PoolConfig config etcd pool
type PoolConfig struct {
	Ctx     context.Context
	TestKey string
	// for health tick
	HealthTick time.Duration
	// etcd Endpoints
	Endpoints []string // store Endpoints
	TLS       *tls.Config
	// Username/Password
	Username string
	Password string

	// dail to etcd
	DialTimeout       time.Duration
	DialKeepAliveTime time.Duration
}

// ClientPool of etcd clientv3, allways choose leader
type ClientPool struct {
	*PoolConfig

	// pool
	clientPool map[string]*c3.Client
	fails      map[string]struct{}
	count      int64

	leader  uint64
	leaderu string
	leaderc *c3.Client

	notifies map[string][]chan error

	mu sync.RWMutex
}

// NewClientv3 return a new v3 client, DON't forget to close it
func (p *ClientPool) NewClientv3(endpoints []string) (*c3.Client, error) {
	log := logutil.DefaultZapLoggerConfig
	log.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	config := c3.Config{
		Endpoints:         endpoints,
		DialTimeout:       p.DialTimeout,
		DialKeepAliveTime: p.DialKeepAliveTime,
		TLS:               p.TLS,
		Context:           p.Ctx,
		Username:          p.Username,
		Password:          p.Password,
		LogConfig:         &log,
	}
	c, err := c3.New(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// NewClientPool return a new etcdc3 pool
func NewClientPool(config *PoolConfig) (*ClientPool, error) {
	p := &ClientPool{
		PoolConfig: config,
	}
	if p.TestKey == "" {
		p.TestKey = defaultTestKey
	}
	if p.DialTimeout <= 0 {
		p.DialTimeout = etcdTimeout
	}
	if p.DialKeepAliveTime <= 0 {
		p.DialKeepAliveTime = keepAliveTime
	}
	if p.Endpoints == nil {
		p.Endpoints = []string{defaultEndpoint}
	}

	return p, p.Init()
}

// Init start health check, connect pool to each etcd member
func (p *ClientPool) Init() error {
	p.fails = make(map[string]struct{})
	p.clientPool = make(map[string]*c3.Client)
	p.notifies = make(map[string][]chan error)

	p.Endpoints = util.Shuffle(p.Endpoints)
	c, err := p.NewClientv3(p.Endpoints)
	if err != nil {
		return err
	}
	p.startHealthCheck()
	return p.checkMembers(c, true)
}

// startHealthCheck start a health check
func (p *ClientPool) startHealthCheck() {
	if p.HealthTick == 0 {
		p.HealthTick = etcdTimeout
	}
	go func() {
		for {
			tick := time.After(p.HealthTick)
			count := atomic.AddInt64(&p.count, 1)
			select {
			case <-p.Ctx.Done():
				return
			case <-tick:
				for url, c := range p.clientPool {
					cctx, cancel := context.WithTimeout(p.Ctx, etcdTimeout)
					_, err := c.Get(cctx, p.TestKey)
					cancel()
					p.mu.Lock()
					_, ok := p.fails[url]
					if err != nil {
						if ecs, ok := p.notifies[url]; ok {
							for _, errc := range ecs {
								errc <- ErrTimeout
								close(errc)
							}
						}
						delete(p.notifies, url)
						if !ok { // first time fail
							logger.Warningf("[etcdc3.poolHealthCheck] test etcd %s error %v\n", url, err)
						}
						p.fails[url] = struct{}{}
					} else if ok { // clear
						logger.Infof("[etcdc3.poolHealthCheck] test etcd ok, %s back to normal!\n", url)
						delete(p.fails, url)
					}
					p.mu.Unlock()
				}
				p.mu.RLock()
				re := len(p.fails) >= len(p.clientPool)/2+1
				p.mu.RUnlock()
				if re {
					logger.Errorf("[etcdc3.poolHealthCheck] half etcd connects fail, check members\n")
					p.checkMembers(nil, true)
				} else if count%60 == 0 {
					p.checkMembers(nil, false)
				}

			}
		}
	}()
}

// Select return a client
func (p *ClientPool) Select() (c *c3.Client) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var (
		url = p.Endpoints[rand.Intn(len(p.Endpoints))]
	)
	for i := 0; i < len(p.Endpoints)*2; i++ {
		if _, ok := p.fails[url]; !ok {
			return p.clientPool[url]
		}
		// again
		url = p.Endpoints[rand.Intn(len(p.Endpoints))]
	}
	return p.leaderc
}

// AllClient return all client's
func (p *ClientPool) AllClient() map[string]*c3.Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	m := make(map[string]*c3.Client, len(p.clientPool))
	for k, v := range p.clientPool {
		m[k] = v
	}
	return m
}

// Close release all resources
func (p *ClientPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, cc := range p.notifies {
		for c := range cc {
			close(cc[c])
		}
	}
	p.notifies = nil
	for _, c := range p.clientPool {
		c.Close()
	}
	p.clientPool = nil
	p.leaderu = ""
	p.leaderc = nil
	p.fails = nil
}

func (p *ClientPool) addNotify(url string, errc chan error) {
	if c, ok := p.notifies[url]; ok {
		p.notifies[url] = append(c, errc)
	} else {
		p.notifies[url] = []chan error{errc}
	}
}

// SelectWithNotify return client with err notify
func (p *ClientPool) SelectWithNotify() (c *c3.Client, errc chan error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var url string
	errc = make(chan error, 1)
	if _, ok := p.fails[p.leaderu]; !ok {
		p.addNotify(p.leaderu, errc)
		return p.leaderc, errc
	}
	for url, c = range p.clientPool {
		if _, ok := p.fails[url]; !ok {
			p.addNotify(url, errc)
			return p.clientPool[url], errc
		}
	}
	p.addNotify(p.leaderu, errc)
	return p.leaderc, errc
}

func (p *ClientPool) checkMembers(c *c3.Client, clean bool) error {
	if c == nil {
		c = p.Select()
	}
	ctx, cancel := context.WithTimeout(p.Ctx, etcdTimeout)
	defer cancel()
	members, err := c.MemberList(ctx)
	if err != nil {
		logger.Infof("[etcdc3] [pool] get member list error %v\n", err)
		return err
	}
	p.leader, err = getStatus(p.Ctx, c, c.Endpoints()[0])
	if err != nil {
		logger.Infof("[etcdc3] [pool] get status error %v\n", err)
		return err
	}

	eps := make([]string, 0, len(members.Members))
	dup := make(map[string]struct{})
	for _, m := range members.Members { // each memebr a url
		for _, url := range m.ClientURLs {
			if _, ok := dup[url]; ok {
				continue
			}
			if p.leader == m.ID {
				if url != p.leaderu {
					logger.Infof("[etcdc3] [pool] leader is %v\n", url)
				}
				p.mu.Lock()
				p.leaderu = url
				p.mu.Unlock()
			}
			eps = append(eps, url)
			dup[url] = struct{}{}
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if clean {
		p.fails = make(map[string]struct{})
	}

	if len(dup) != len(p.clientPool) {
		logger.Infof("[etcdc3] [pool] connect to member list %v\n", eps)
	}

	for k, c := range p.clientPool {
		if _, ok := dup[k]; !ok {
			logger.Infof("[etcdc3] [pool] will delete client %s\n", k)
			// delete
			c.Close()
			delete(p.clientPool, k)
			delete(p.fails, k)
		}
	}

	var (
		errCount = 0
		e        error
	)
	for k := range dup {
		c, ok := p.clientPool[k]
		if !ok {
			logger.Infof("[etcdc3] [pool] will connect to %v\n", k)
			// add
			c, err = p.NewClientv3([]string{k})
			if err != nil {
				errCount++
				e = err
				logger.Errorf("[etcdc3] [pool] connect to %v with error %v\n", k, err)
				continue
			}
			p.clientPool[k] = c
		}
		if k == p.leaderu {
			p.leaderc = c
		}
	}
	p.Endpoints = make([]string, 0, len(p.clientPool))
	for k := range p.clientPool {
		p.Endpoints = append(p.Endpoints, k)
	}
	if errCount == len(dup) { // in initial
		return e
	}

	return nil
}

func getStatus(ctx context.Context, c *c3.Client, ep string) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, etcdTimeout)
	defer cancel()
	s, err := c.Status(ctx, ep)
	if err != nil {
		return 0, err
	}
	return s.Leader, nil
}

const (
	etcdTimeout     = 5 * time.Second
	keepAliveTime   = 60 * time.Second
	defaultEndpoint = "http://localhost:2379"
)

var (
	defaultTestKey = "/foo"
	logger         *logging.Logger

	// ErrTimeout when waching wait timeout
	ErrTimeout = errors.New("watching wait timeout")
)
