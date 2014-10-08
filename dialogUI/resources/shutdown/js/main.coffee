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

powerchoose = null
DEBUG = DCore.Shutdown.is_debug()
restack_interval = null

main = ->
    enableZoneDetect(false)
    powerchoose = new PowerChoose()
    document.body.addEventListener("keydown",(e)->
        powerchoose?.keydown(e.which)
    )

    powerchoose.frame_build()

    if !DEBUG
        restack_interval = setInterval(=>
            DCore.Shutdown.restack()
        ,50)

DCore.signal_connect('workarea_size_changed', (alloc)->
    echo "primary_size_changed:#{alloc.width}*#{alloc.height}(#{alloc.x},#{alloc.y})"

    document.body.addEventListener("click", (e)->
        console.debug "body click"
        destory_all()
    )

    main()
)

DCore.Shutdown.emit_webview_ok()
