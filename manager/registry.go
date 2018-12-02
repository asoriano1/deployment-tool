package main

import (
	"fmt"
	"log"

	"code.linksmart.eu/dt/deployment-tool/manager/model"
)

const (
	SystemLogsKey = "SYS"
)

type registry struct {
	orders  map[string]*Order
	targets map[string]*Target
}

//
// Order
//
type Order struct {
	model.Header `yaml:",inline"`
	Stages       model.Stages `json:"stages"`
	Target       struct {
		IDs  []string `json:"ids"`
		Tags []string `json:"tags"`
	} `json:"targets"`
	Receivers []string `json:"receivers"`
}

func (o Order) Validate() error {
	if len(o.Stages.Transfer)+len(o.Stages.Install)+len(o.Stages.Run) == 0 {
		return fmt.Errorf("empty stages")
	}
	return nil
}

//
// TARGET
//
type Target struct {
	Tags           []string
	Tasks          map[string]*Task
	LastLogRequest model.UnixTimeType
}

func newTarget() *Target {
	return &Target{
		Tasks: make(map[string]*Task),
	}
}

func (t *Target) initTask(id string) {
	if _, found := t.Tasks[id]; !found {
		t.Tasks[id] = new(Task)
	}
}

type Task struct {
	Stages  StageLogs
	Updated model.UnixTimeType
}

func (t *Task) GetStageLog(stage string) *StageLog {
	switch stage {
	case model.StageAssemble:
		return &t.Stages.Assemble
	case model.StageTransfer:
		return &t.Stages.Transfer
	case model.StageInstall:
		return &t.Stages.Install
	case model.StageTest:
		return &t.Stages.Test
	case model.StageRun:
		return &t.Stages.Run
	}
	log.Println("ERROR: Unknown/unsupported stage:", stage)
	return &StageLog{}
}

type StageLogs struct {
	Assemble StageLog
	Transfer StageLog
	Install  StageLog
	Test     StageLog
	Run      StageLog
}

type StageLog struct {
	Logs map[string][]Log `json:",omitempty"'`
}

type Log struct {
	Output string
	Error  bool `json:",omitempty"'`
	Time   model.UnixTimeType
}

func (s *StageLog) InsertLogs(l model.Log) {
	if l.Command == "" {
		l.Command = SystemLogsKey
	}
	if s.Logs == nil {
		s.Logs = make(map[string][]Log)
	}
	// first insertion
	if len(s.Logs[l.Command]) == 0 {
		s.Logs[l.Command] = append(s.Logs[l.Command], Log{l.Output, l.Error, l.Time})
		return
	}

	i := 0
	for ; i < len(s.Logs[l.Command]); i++ {
		log := s.Logs[l.Command][i]
		// discard if duplicate
		if log.Time == l.Time && log.Output == l.Output {
			return
		}
		// find the index where it should be inserted
		if i == len(s.Logs[l.Command])-1 || (l.Time >= log.Time && l.Time < s.Logs[l.Command][i+1].Time) {
			i++
			break
		}
	}
	// append to the end
	if i == len(s.Logs[l.Command]) {
		s.Logs[l.Command] = append(s.Logs[l.Command], Log{l.Output, l.Error, l.Time})
		return
	}
	// insert in the middle
	s.Logs[l.Command] = append(s.Logs[l.Command], Log{})
	copy(s.Logs[l.Command][i+1:], s.Logs[l.Command][i:])
	s.Logs[l.Command][i] = Log{l.Output, l.Error, l.Time}
}

func (s *StageLog) Flush() {
	*s = StageLog{}
}
