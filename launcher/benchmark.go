package launcher

import (
	"fmt"
	"github.com/HouzuoGuo/laitos/inet"
	"github.com/HouzuoGuo/laitos/misc"
	"math/rand"
	"net"
	"net/http"
	_ "net/http/pprof" // pprof package has an init routine that installs profiler API handlers
	"net/smtp"
	"strconv"
	"sync/atomic"
	"time"
)

type Benchmark struct {
	Config      *Config     // Config is an initialised configuration structure that provides for all daemons involved in benchmark.
	DaemonNames []string    // DaemonNames is a list of daemons that have already started and waiting to run benchmark.
	Logger      misc.Logger // Logger is specified by caller if the caller wishes.
	HTTPPort    int         // HTTPPort is to be served by net/http/pprof on an HTTP server running on localhost.
	Stop        bool        // Stop, if true, will soon terminate ongoing benchmark. It may be reset to false in preparation for a new benchmark run.
}

/*
RunBenchmarkAndProfiler starts benchmark immediately and continually reports progress via logger. The function kicks off
more goroutines for benchmarking individual daemons, and therefore does not block caller.

Benchmark cases usually uses randomly generated data and do not expect a normal response. Therefore, they serve well as
fuzzy tests too.

The function assumes that daemons are already started and ready to receive requests, therefore caller may wish to
consider waiting a short while for daemons to settle before running this benchmark routine.
*/
func (bench *Benchmark) RunBenchmarkAndProfiler() {
	// Expose profiler APIs via HTTP server running on a fixed port number on localhost
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("localhost:%d", bench.HTTPPort), nil); err != nil {
			bench.Logger.Abort("RunBenchmarkAndProfiler", "", err, "failed to start profiler HTTP server")
		}
	}()
	for _, daemonName := range bench.DaemonNames {
		// Kick off benchmarks
		switch daemonName {
		case DNSDName:
			go bench.BenchmarkDNSDaemon()
		case HTTPDName:
			go bench.BenchmarkHTTPSDaemon()
		case InsecureHTTPDName:
			go bench.BenchmarkHTTPDaemon()
		case MaintenanceName:
			// There is no benchmark for maintenance daemon
		case PlainSocketName:
			go bench.BenchmarkPlainSocketDaemon()
		case SMTPDName:
			go bench.BenchmarkSMTPDaemon()
		case SOCKDName:
			go bench.BenchmarkSockDaemon()
		case TelegramName:
			// There is no benchmark for telegram daemon
		}
	}
}

/*
ReportRatePerSecond runs the input function (which most likely runs indefinitely) and logs rate of invocation of a
trigger function (fed to the input function) every second. The function blocks caller as long as input function
continues to run.
*/
func (bench *Benchmark) reportRatePerSecond(loop func(func()), name string, logger misc.Logger) {
	unitTimeSec := 1
	ticker := time.NewTicker(time.Duration(unitTimeSec) * time.Second)

	var counter, total int64
	go func() {
		for {
			if bench.Stop {
				return
			}
			<-ticker.C
			counter := atomic.LoadInt64(&counter)
			logger.Info("reportRatePerSecond", name, nil, "%d/s (total %d)", atomic.SwapInt64(&counter, 0)/int64(unitTimeSec), counter)
		}
	}()
	loop(func() {
		atomic.AddInt64(&counter, 1)
		atomic.AddInt64(&total, 1)
	})
}

// BenchmarkDNSDaemon continually sends DNS queries via both TCP and UDP in a sequential manner.
func (bench *Benchmark) BenchmarkDNSDaemon() {
	var doUDP bool

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(bench.Config.GetDNSD().UDPPort))
	if err != nil {
		bench.Logger.Panic("BenchmarkDNSDaemon", "", err, "failed to init UDP address")
		return
	}
	tcpPort := bench.Config.GetDNSD().TCPPort

	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()

			buf := make([]byte, 32*1024)
			if _, err := rand.Read(buf); err != nil {
				bench.Logger.Panic("BenchmarkDNSDaemon", "", err, "failed to acquire random bytes")
				return
			}

			if doUDP {
				doUDP = false
				clientConn, err := net.DialUDP("udp", nil, udpAddr)
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			} else {
				doUDP = true
				clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(tcpPort))
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			}
		}
	}, "BenchmarkDNSDaemon", bench.Logger)
}

// BenchmarkHTTPDaemonn continually sends HTTP requests in a sequential manner.
func (bench *Benchmark) BenchmarkHTTPDaemon() {
	allRoutes := make([]string, 0, 32)
	for installedRoute := range bench.Config.GetHTTPD().AllRateLimits {
		allRoutes = append(allRoutes, installedRoute)
	}
	if len(allRoutes) == 0 {
		bench.Logger.Abort("BenchmarkHTTPDaemon", "", nil, "HTTP daemon does not any route at all, cannot benchmark it.")
	}
	urlTemplate := fmt.Sprintf("http://localhost:%d%%s", bench.Config.GetHTTPD().PlainPort)

	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()
			inet.DoHTTP(inet.HTTPRequest{TimeoutSec: 3}, fmt.Sprintf(urlTemplate, allRoutes[rand.Intn(len(allRoutes))]))
		}
	}, "BenchmarkHTTPDaemon", bench.Logger)
}

// BenchmarkHTTPDaemonn continually sends HTTPS requests in a sequential manner.
func (bench *Benchmark) BenchmarkHTTPSDaemon() {
	allRoutes := make([]string, 0, 32)
	for installedRoute := range bench.Config.GetHTTPD().AllRateLimits {
		allRoutes = append(allRoutes, installedRoute)
	}
	if len(allRoutes) == 0 {
		bench.Logger.Abort("BenchmarkHTTPSDaemon", "", nil, "HTTP daemon does not any route at all, cannot benchmark it.")
	}
	urlTemplate := fmt.Sprintf("https://localhost:%d%%s", bench.Config.GetHTTPD().PlainPort)

	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()
			inet.DoHTTP(inet.HTTPRequest{TimeoutSec: 3}, fmt.Sprintf(urlTemplate, allRoutes[rand.Intn(len(allRoutes))]))
		}
	}, "BenchmarkHTTPSDaemon", bench.Logger)

}

// BenchmarkPlainSocketDaemon continually sends toolbox commands via both TCP and UDP in a sequential manner.
func (bench *Benchmark) BenchmarkPlainSocketDaemon() {
	var doUDP bool

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(bench.Config.GetPlainSocketDaemon().UDPPort))
	if err != nil {
		bench.Logger.Panic("BenchmarkPlainSocketDaemon", "", err, "failed to init UDP address")
		return
	}
	tcpPort := bench.Config.GetPlainSocketDaemon().TCPPort

	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()

			buf := make([]byte, 32*1024)
			if _, err := rand.Read(buf); err != nil {
				bench.Logger.Panic("BenchmarkPlainSocketDaemon", "", err, "failed to acquire random bytes")
				return
			}

			if doUDP {
				doUDP = false
				clientConn, err := net.DialUDP("udp", nil, udpAddr)
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			} else {
				doUDP = true
				clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(tcpPort))
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			}
		}
	}, "BenchmarkPlainSocketDaemon", bench.Logger)
}

// BenchmarkSMTPDaemon continually sends emails in a sequential manner.
func (bench *Benchmark) BenchmarkSMTPDaemon() {
	port := bench.Config.GetMailDaemon().Port
	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()

			buf := make([]byte, 32*1024)
			if _, err := rand.Read(buf); err != nil {
				bench.Logger.Panic("BenchmarkSMTPDaemon", "", err, "failed to acquire random bytes")
				return
			}

			smtp.SendMail(fmt.Sprintf("localhost:%d", port), nil, "ClientFrom@localhost", []string{"ClientTo@does-not-exist.com"}, buf)
		}
	}, "BenchmarkSMTPDaemon", bench.Logger)

}

// BenchmarkSockDaemon continually sends packets via both TCP and UDP in a sequential manner.
func (bench *Benchmark) BenchmarkSockDaemon() {
	var doUDP bool

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(bench.Config.GetSockDaemon().UDPPort))
	if err != nil {
		bench.Logger.Panic("BenchmarkSockDaemon", "", err, "failed to init UDP address")
		return
	}
	tcpPort := bench.Config.GetSockDaemon().TCPPort

	rand.Seed(time.Now().UnixNano())

	bench.reportRatePerSecond(func(trigger func()) {
		for {
			if bench.Stop {
				return
			}
			trigger()

			buf := make([]byte, 32*1024)
			if _, err := rand.Read(buf); err != nil {
				bench.Logger.Panic("BenchmarkSockDaemon", "", err, "failed to acquire random bytes")
				return
			}

			if doUDP {
				doUDP = false
				clientConn, err := net.DialUDP("udp", nil, udpAddr)
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			} else {
				doUDP = true
				clientConn, err := net.Dial("tcp", "127.0.0.1:"+strconv.Itoa(tcpPort))
				if err != nil {
					continue
				}
				if err := clientConn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
					clientConn.Close()
					continue
				}
				if _, err := clientConn.Write(buf); err != nil {
					clientConn.Close()
					continue
				}
				clientConn.Close()
			}
		}
	}, "BenchmarkSockDaemon", bench.Logger)
}
