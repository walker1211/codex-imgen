package config

import "time"

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Storage   StorageConfig   `yaml:"storage"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
	Backend   BackendConfig   `yaml:"backend"`
	Email     EmailConfig     `yaml:"email"`
}

type ServerConfig struct {
	Listen       string        `yaml:"listen"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

type StorageConfig struct {
	DataDir    string `yaml:"data_dir"`
	SQLitePath string `yaml:"sqlite_path"`
}

type SchedulerConfig struct {
	GlobalMaxConcurrency  int           `yaml:"global_max_concurrency"`
	DefaultJobConcurrency int           `yaml:"default_job_concurrency"`
	MaxJobConcurrency     int           `yaml:"max_job_concurrency"`
	MaxCountPerJob        int           `yaml:"max_count_per_job"`
	MaintenanceInterval   time.Duration `yaml:"maintenance_interval"`
	TaskLeaseTimeout      time.Duration `yaml:"task_lease_timeout"`
	MaxAttempts           int           `yaml:"max_attempts"`
}

type BackendConfig struct {
	Type    string        `yaml:"type"`
	Command string        `yaml:"command"`
	Model   string        `yaml:"model"`
	CWD     string        `yaml:"cwd"`
	Timeout time.Duration `yaml:"timeout"`
	Prompt  PromptConfig  `yaml:"prompt"`
}

type PromptConfig struct {
	Prefix  string `yaml:"prefix"`
	Prelude string `yaml:"prelude"`
}

type EmailConfig struct {
	Enabled       bool          `yaml:"enabled"`
	SMTPHost      string        `yaml:"smtp_host"`
	SMTPPort      int           `yaml:"smtp_port"`
	From          string        `yaml:"from"`
	To            string        `yaml:"to"`
	Timeout       time.Duration `yaml:"timeout"`
	RetryTimes    int           `yaml:"retry_times"`
	RetryWaitTime time.Duration `yaml:"retry_wait_time"`
	UseProxy      bool          `yaml:"use_proxy"`
	SMTPAuthCode  string        `yaml:"-"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Listen:       "127.0.0.1:18080",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Storage: StorageConfig{},
		Scheduler: SchedulerConfig{
			GlobalMaxConcurrency:  10,
			DefaultJobConcurrency: 2,
			MaxJobConcurrency:     10,
			MaxCountPerJob:        10,
			MaintenanceInterval:   5 * time.Minute,
			TaskLeaseTimeout:      30 * time.Minute,
			MaxAttempts:           3,
		},
		Backend: BackendConfig{
			Type:    "built_in_codex",
			Command: "codex",
			Timeout: 90 * time.Second,
			Prompt: PromptConfig{
				Prefix: "$imagegen",
			},
		},
		Email: EmailConfig{},
	}
}
