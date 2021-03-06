package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Process ... Defines a process running in octyne.
type Process struct {
	ServerConfig
	Name    string
	Command *exec.Cmd
	Online  int // 0 for offline, 1 for online, 2 for failure
	Output  *io.PipeReader
	Input   *io.PipeWriter
	Stdin   io.WriteCloser
}

// RunProcess ... Runs a process.
func RunProcess(name string, config ServerConfig, connector *Connector) *Process {
	// Create the process.
	output, input := io.Pipe()
	process := &Process{
		Name:         name,
		Online:       0,
		ServerConfig: config,
		Output:       output,
		Input:        input,
	}
	// Run the command.
	process.StartProcess()
	connector.AddProcess(process)
	return process
}

// StartProcess ... Starts the process.
func (process *Process) StartProcess() error {
	name := process.Name
	log.Println("Starting server (" + name + ")")
	// Determine the command which should be run by Go and change the working directory.
	cmd := strings.Split(process.ServerConfig.Command, " ")
	command := exec.Command(cmd[0], cmd[1:]...)
	command.Dir = process.Directory
	// Run the command after retrieving the standard out, standard in and standard err.
	process.Stdin, _ = command.StdinPipe()
	command.Stdout = process.Input
	command.Stderr = command.Stdout // We want the stderr and stdout to go to the same pipe.
	err := command.Start()
	// Check for errors.
	process.Online = 2
	if err != nil {
		log.Println("Failed to start server (" + name + ")! The following error occured:\n" + err.Error())
	} else if _, err := os.FindProcess(command.Process.Pid); err != nil /* Windows */ ||
		// command.Process.Signal(syscall.Signal(0)) != nil /* Unix, disabled */ ||
		command.ProcessState != nil /* Universal */ {
		log.Println("Server (" + name + ") is not online!")
		// Get the stdout and stderr and log..
		var stdout bytes.Buffer
		stdout.ReadFrom(process.Output)
		log.Println("Output:\n" + stdout.String())
	} else {
		log.Println("Started server (" + name + ") with PID " + strconv.Itoa(command.Process.Pid))
		process.SendConsoleOutput("[Octyne] Started server " + name)
		process.Online = 1
	}
	// Update and return.
	process.Command = command
	go process.MonitorProcess()
	return err
}

// StopProcess ... Stops the process.
func (process *Process) StopProcess() {
	log.Println("Stopping server (" + process.Name + ")")
	process.SendConsoleOutput("[Octyne] Stopping server " + process.Name)
	command := process.Command
	// Stop the command.
	command.Process.Kill()
	process.Online = 0
	process.SendConsoleOutput("[Octyne] Stopped server " + process.Name)
}

// SendCommand ... Sends an input to stdin of the process.
func (process *Process) SendCommand(command string) {
	fmt.Fprintln(process.Stdin, command)
}

// SendConsoleOutput ... Sends console output to the stdout of the process.
func (process *Process) SendConsoleOutput(command string) {
	go fmt.Fprintln(process.Input, command)
}

// MonitorProcess ... Monitors the process and automatically marks it as offline/online.
func (process *Process) MonitorProcess() error {
	defer (func() {
		if e := recover(); e != nil {
			log.Println(e) // In case of nil pointer exception.
		}
	})()
	// Exit immediately if there is no process.
	if process.Command.Process == nil {
		return nil
	}
	// Wait for the command to finish execution.
	err := process.Command.Wait()
	// Mark as offline appropriately.
	if process.Command.ProcessState.Success() || process.Online == 0 {
		process.Online = 0
		log.Println("Server (" + process.Name + ") has stopped.")
		process.SendConsoleOutput("[Octyne] Server " + process.Name + " has stopped.")
	} else {
		process.Online = 2
		process.SendConsoleOutput("[Octyne] Server " + process.Name + " has crashed!")
		log.Println("Server (" + process.Name + ") has crashed!")
		// TODO: Implement a limit of crash restarts before letting the server stop.
		process.SendConsoleOutput("[Octyne] Restarting server " + process.Name + " due to default behaviour.")
		process.StartProcess()
	}
	return err
}
