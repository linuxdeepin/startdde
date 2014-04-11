#Copyright (c) 2011 ~ 2014 Deepin, Inc.
#              2011 ~ 2014 bluth
#
#encoding: utf-8
#Author:      bluth <yuanchenglu@linuxdeepin.com>
#Maintainer:  bluth <yuanchenglu@linuxdeepin.com>
#
#This program is free software; you can redistribute it and/or modify
#it under the terms of the GNU General Public License as published by
#the Free Software Foundation; either version 3 of the License, or
#(at your option) any later version.
#
#This program is distributed in the hope that it will be useful,
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU General Public License for more details.
#
#You should have received a copy of the GNU General Public License
#along with this program; if not, see <http://www.gnu.org/licenses/>.

SessionManager = "com.deepin.SessionManager",

power_request = (power) ->
    # option = ["lock","suspend","logout","restart","shutdown"]
    try
        dbus_power = DCore.DBus.session(SessionManager)
        echo dbus_power
    catch e
        echo "dbus_power error:#{e}"
    if not dbus_power? then return
    document.body.style.cursor = "wait" if power isnt "suspend" and power isnt "lock"
    echo "Warning: The system will request ----#{power}----"
    switch power
        when "lock" then dbus_power.RequestLock()
        when "suspend" then dbus_power.RequestSuspend()
        when "logout" then dbus_power.RequestLogout()
        when "restart" then dbus_power.RequestReboot()
        when "shutdown" then dbus_power.RequestShutdown()
        else return

power_can = (power) ->
    try
        dbus_power = DCore.DBus.session(SessionManager)
        echo dbus_power
    catch e
        echo "dbus_power error:#{e}"
    if not dbus_power? then return
    result = true
    switch power
        when "lock" then result = true
        when "suspend" then result = dbus_power.CanSuspend_sync()
        when "logout" then result = dbus_power.CanLogout_sync()
        when "restart" then result = dbus_power.CanReboot_sync()
        when "shutdown" then result = dbus_power.CanShutdown_sync()
        else result = false
    echo "power_can : -----------Can_#{power} :#{result}------------"
    if result is undefined then result = true
    return result

power_force = (power) ->
    try
        dbus_power = DCore.DBus.session(SessionManager)
        echo dbus_power
    catch e
        echo "dbus_power error:#{e}"
    if not dbus_power? then return
    # option = ["lock","suspend","logout","restart","shutdown"]
    echo dbus_power
    document.body.style.cursor = "wait" if power isnt "suspend" and power isnt "lock"
    echo "Warning: The system will ----#{power}---- Force!!"
    switch power
        when "lock" then dbus_power.RequestLock()
        when "suspend" then dbus_power.RequestSuspend()
        when "logout" then dbus_power.ForceLogout()
        when "restart" then dbus_power.ForceReboot()
        when "shutdown" then dbus_power.ForceShutdown()
        else return


