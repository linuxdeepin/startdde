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

APP_NAME = "Shutdown"
DEFAULT_BG = "/usr/share/backgrounds/default_background.jpg"
option = ["shutdown","restart","suspend","lock","user_switch","logout"]
option_text = [_("Shut down"),_("Restart"),_("Suspend"),_("Lock"),_("Switch user"),_("Log out")]
message_text = {}

get_accounts_lenght = ->
    dbus = DCore.DBus.sys("com.deepin.daemon.Accounts")
    length = dbus.UserList.length
    if dbus.AllowGuest then ++length
    return length

if get_accounts_lenght() < 2
    index = i for opt,i in option when opt is "user_switch"
    echo "splice : #{index}:#{option[index]}"
    option.splice(index,1)
    option_text.splice(index,1)

zoneDBus = null
enableZoneDetect = (enable) ->
    ZONE = "com.deepin.daemon.Zone"
    try
        if not zoneDBus?
            zoneDBus = DCore.DBus.session(ZONE)
        zoneDBus.EnableZoneDetected_sync(enable)
    catch e
        echo "zoneDBus #{ZONE} error : #{e}"

destory_all = ->
    console.log "destory_all"
    clearInterval(restack_interval)
    enableZoneDetect(true)
    DCore.Shutdown.quit()

TIME_SHOW = 500
showAnimation =(el,t)->
    _b = document.body
    _b.style.display = "none"
    jQuery(_b).fadeIn(t)
