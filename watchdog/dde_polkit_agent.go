/**
 * Copyright (C) 2017 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package watchdog

import (
	"dbus/org/freedesktop/login1"
	"dbus/org/freedesktop/policykit1"
	"fmt"
	"os/exec"
	"pkg.deepin.io/lib/dbus"
	"pkg.deepin.io/lib/utils"
)

type polkitSubject struct {
	Kind    string
	Details map[string]dbus.Variant
}

const (
	ddePolkitAgentCommand  = "/usr/lib/polkit-1-dde/dde-polkit-agent"
	ddePolkitAgentDBusPath = "/com/deepin/polkit/AuthenticationAgent"
)

func isDDEPolkitAgentRunning() bool {
	// only listen dde polkit agent
	if !utils.IsFileExist(ddePolkitAgentCommand) {
		return true
	}

	polkit1, err := policykit1.NewAuthority("org.freedesktop.PolicyKit1",
		"/org/freedesktop/PolicyKit1/Authority")
	if err != nil {
		logger.Warning("Failed to create polkit authority:", err)
		return true
	}
	defer policykit1.DestroyAuthority(polkit1)

	var subject = polkitSubject{
		Kind:    "unix-session",
		Details: make(map[string]dbus.Variant),
	}
	subject.Details["session-id"] = dbus.MakeVariant(getCurrentSessionID())
	err = polkit1.RegisterAuthenticationAgent(&subject,
		"",
		ddePolkitAgentDBusPath)
	if err != nil {
		logger.Debug("Failed to registe dde polkit agent:", err)
		return true
	}

	logger.Debug("dde polkit agent not running, will launch it...")
	// if register successfully, the dde polkit agent not running
	err = polkit1.UnregisterAuthenticationAgent(&subject, ddePolkitAgentDBusPath)
	if err != nil {
		logger.Warning("Failed to unregister dde polkit agent:", err)
		// TODO: how to handle unregister failure?
	}
	return false
}

func launchDDEPolkitAgent() error {
	var cmd = exec.Command(ddePolkitAgentCommand)
	err := cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Warning("Failed to wait dde polkit agent exec:", err)
		}
	}()
	return nil
}

func newDDEPolkitAgent() *taskInfo {
	return newTaskInfo("dde-polkit-agent", isDDEPolkitAgentRunning, launchDDEPolkitAgent)
}

func getCurrentSessionID() string {
	self, err := login1.NewSession("org.freedesktop.login1", "/org/freedesktop/login1/session/self")
	if err != nil {
		fmt.Println("Failed to create self session:", err)
		return ""
	}
	defer login1.DestroySession(self)

	return self.Id.Get()
}
