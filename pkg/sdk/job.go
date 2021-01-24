package sdk

import (
	"golang.org/x/xerrors"
	"log"
)

type Job struct {
	ContextConfigFunc func() (string, string, *AssumeRoleConfig)
}

func NewJob(f func() (string, string, *AssumeRoleConfig)) *Job {
	return &Job{ContextConfigFunc: f}
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
	region, profile, assumeRoleConfig := s.ContextConfigFunc()

	sess, creds := AWSCredsFromConfig(region, profile, assumeRoleConfig)

	return &Context{Sess: sess, Creds: creds}
}
