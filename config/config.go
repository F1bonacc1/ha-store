package config

import (
	"flag"
	"os"
	"strconv"
	"time"
)

type Config struct {
	NATSURL        string
	Port           string
	Replicas       int
	TLSCertFile    string
	TLSKeyFile     string
	TLSCAFile      string
	ThrottleSpeed  int64         // bytes per second
	UploadDeadline time.Duration // timeout for upload/download operations
	DeleteDeadline time.Duration // timeout for delete operations
}

var (
	natsURLFlag        string
	portFlag           string
	replicasFlag       int
	tlsCertFileFlag    string
	tlsKeyFileFlag     string
	tlsCAFileFlag      string
	throttleSpeedFlag  int
	uploadDeadlineFlag int
	deleteDeadlineFlag int
)

func init() {
	flag.StringVar(&natsURLFlag, "nats-url", "", "NATS URL")
	flag.StringVar(&portFlag, "port", "", "Server Port")
	flag.IntVar(&replicasFlag, "replicas", 0, "Number of replicas for object store")
	flag.StringVar(&tlsCertFileFlag, "tls-cert", "", "TLS certificate file path")
	flag.StringVar(&tlsKeyFileFlag, "tls-key", "", "TLS private key file path")
	flag.StringVar(&tlsCAFileFlag, "tls-ca", "", "CA certificate file for client verification")
	flag.IntVar(&throttleSpeedFlag, "throttle-speed", 0, "Upload throttle speed in MB/s (default: 150)")
	flag.IntVar(&uploadDeadlineFlag, "upload-deadline", 0, "Upload/download context deadline in seconds (default: 600)")
	flag.IntVar(&deleteDeadlineFlag, "delete-deadline", 0, "Delete context deadline in seconds (default: 60)")
}

func Load() *Config {
	if !flag.Parsed() {
		flag.Parse()
	}

	natsURL := natsURLFlag
	port := portFlag
	replicas := replicasFlag
	tlsCertFile := tlsCertFileFlag
	tlsKeyFile := tlsKeyFileFlag
	tlsCAFile := tlsCAFileFlag
	throttleSpeed := throttleSpeedFlag
	uploadDeadline := uploadDeadlineFlag
	deleteDeadline := deleteDeadlineFlag

	// Fallback to Env
	if natsURL == "" {
		natsURL = os.Getenv("NATS_URL")
		if natsURL == "" {
			natsURL = "nats://localhost:4222"
		}
	}

	if port == "" {
		port = os.Getenv("PORT")
		if port == "" {
			port = "8090"
		}
	}

	if replicas == 0 {
		envReplicas := os.Getenv("REPLICAS")
		if envReplicas != "" {
			if r, err := strconv.Atoi(envReplicas); err == nil {
				replicas = r
			}
		}
		if replicas == 0 {
			replicas = 1
		}
	}

	if tlsCertFile == "" {
		tlsCertFile = os.Getenv("TLS_CERT_FILE")
	}
	if tlsKeyFile == "" {
		tlsKeyFile = os.Getenv("TLS_KEY_FILE")
	}
	if tlsCAFile == "" {
		tlsCAFile = os.Getenv("TLS_CA_FILE")
	}

	if throttleSpeed == 0 {
		envThrottleSpeed := os.Getenv("THROTTLE_SPEED")
		if envThrottleSpeed != "" {
			if t, err := strconv.Atoi(envThrottleSpeed); err == nil {
				throttleSpeed = t
			}
		}
		if throttleSpeed == 0 {
			throttleSpeed = 150 // 150 MB/s default
		}
	}

	if uploadDeadline == 0 {
		envUploadDeadline := os.Getenv("UPLOAD_DEADLINE")
		if envUploadDeadline != "" {
			if u, err := strconv.Atoi(envUploadDeadline); err == nil {
				uploadDeadline = u
			}
		}
		if uploadDeadline == 0 {
			uploadDeadline = 600 // 600 seconds default
		}
	}

	if deleteDeadline == 0 {
		envDeleteDeadline := os.Getenv("DELETE_DEADLINE")
		if envDeleteDeadline != "" {
			if d, err := strconv.Atoi(envDeleteDeadline); err == nil {
				deleteDeadline = d
			}
		}
		if deleteDeadline == 0 {
			deleteDeadline = 60 // 60 seconds default
		}
	}

	return &Config{
		NATSURL:        natsURL,
		Port:           port,
		Replicas:       replicas,
		TLSCertFile:    tlsCertFile,
		TLSKeyFile:     tlsKeyFile,
		TLSCAFile:      tlsCAFile,
		ThrottleSpeed:  int64(throttleSpeed) * 1024 * 1024, // Convert MB/s to bytes/s
		UploadDeadline: time.Duration(uploadDeadline) * time.Second,
		DeleteDeadline: time.Duration(deleteDeadline) * time.Second,
	}
}
