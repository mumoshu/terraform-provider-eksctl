package sdk

import (
	"golang.org/x/xerrors"
	"log"
)

type Job struct {
	Conf *Config
}

func NewJob(conf *Config) *Job {
	return &Job{Conf: conf}
}

func (s *Job) Task(name string, f func(*Context) error) (err error) {
	defer func() {
		if err != nil {
			log.Printf("Task %s failed due to error: %+v", name, err)

			err = xerrors.Errorf("task %s: %w", name, err)

			return
		}
		if e := recover(); e != nil {
			log.Printf("Task %s failed due to panic: %+v", name, e)

			err = xerrors.Errorf("task %s: %w", name, e)
		}
	}()

	ctx := s.newContext()

	return f(ctx)
}

func (s *Job) newContext() *Context {
	return ContextConfig(s.Conf)
}

func ContextConfig(conf *Config) *Context {
	sess, creds := AWSCredsFromConfig(conf)

	return &Context{
		Sess:  sess,
		Creds: creds,
	}
}
