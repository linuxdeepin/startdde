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

option = ["shutdown","restart","lock","suspend","logout"]
option_text = [_("Shut down"),_("Restart"),_("Lock"),_("Suspend"),_("Log out")]
message_text = {}
message_text["systemUpdate"] = _("Your system is updating.please donnot poweroff...")
message_text["default"] = _("Are you sure to do it?")

timeId = null

destory_all = ->
    clearInterval(timeId) if timeId
    DCore.Shutdown.quit()

document.body.style.height = window.innerHeight
document.body.style.width = window.innerWidth

TIME_SHOW = 500
showAnimation =(el,t)->
    _b = document.body
    _b.style.display = "none"
    jQuery(_b).fadeIn(t)

