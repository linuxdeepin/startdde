#Copyright (c) 2012 ~ 2013 Deepin, Inc.
#              2012 ~ 2013 bluth
#
#encoding: utf-8
#Author:      bluth <\yuanchenglu@linuxdeepin.com>
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

powerchoose = new PowerChoose()
powerchoose.frame_build()
document.body.appendChild(powerchoose.element)

document.body.addEventListener("keydown",(e)->
    if powerchoose then powerchoose.keydown(e.which)
    else if confirmdialog then confirmdialog.keydown(e.which)
    )

document.body.addEventListener("click",->
    if !frame_click
        destory_all()
    frame_click = false
    )
