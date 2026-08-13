// Package botcontrol implements the protocol between a router's polling
// agent and the VPS control-server, and (agent.go) the router-side
// client. The control-server itself (Telegram bot, HTTP server, command
// queue) is a separate binary -- it runs on different infrastructure with
// different concerns, so it doesn't belong in the router's single binary
// the way the agent does.
package botcontrol

import "time"

// Action names a unit of work an agent can execute. Kept as plain
// strings (not run through a Go type per action) since both sides only
// ever need to route on the string and log it -- a heavier action
// registry would be premature for the command set this project has.
const (
	ActionStatus        = "status"
	ActionSwitchPrimary = "switch_primary"
	ActionSwitchBackup  = "switch_backup"
	ActionProfileList   = "profile_list"
	ActionSubSetURL     = "sub_seturl"
	ActionSubRefresh    = "sub_refresh"
	ActionSubList       = "sub_list"
	ActionSubSetPrimary = "sub_setprimary"
	ActionSubSetBackup  = "sub_setbackup"
)

// Command is a single unit of work queued for a router by the control
// server, to be fetched and executed by that router's agent.
type Command struct {
	ID     string    `json:"id"`
	Action string    `json:"action"`
	Args   []string  `json:"args,omitempty"`
	Queued time.Time `json:"queued"`
}

// Result is what the agent posts back after executing a Command.
type Result struct {
	CommandID string    `json:"command_id"`
	Output    string    `json:"output"`
	Err       string    `json:"error,omitempty"`
	Completed time.Time `json:"completed"`
}

// PollRequest is what the agent sends to ask for queued work.
type PollRequest struct {
	RouterID string `json:"router_id"`
}

// PollResponse carries at most one queued command -- the agent executes
// it, posts the Result, and polls again for the next one rather than
// being handed a batch.
type PollResponse struct {
	Command *Command `json:"command,omitempty"`
}

// ResultRequest is what the agent posts after executing a command.
type ResultRequest struct {
	RouterID string `json:"router_id"`
	Result   Result `json:"result"`
}
