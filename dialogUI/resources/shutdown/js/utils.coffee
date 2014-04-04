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

confirmdialog = null
powerchoose = null

frame_click = false
option = ["logout","shutdown","restart","suspend","lock"]
option_text = [_("Logout"),_("Shutdown"),_("Restart"),_("Suspend"),_("Lock")]
option_text_force = [_("Force Log out"),_("Force Shut down"),_("Force Restart"),_("Suspend"),_("Lock")]
message_text = [
    _("You will be automatically logged out in %1 seconds."),
    _("The system will shut down in %1 seconds."),
    _("The system will restart in %1 seconds."),
    _("The system will be suspended in %1 seconds."),
    _("The system will be locked in %1 seconds.")
]

timeId = null

destory_all = ->
    clearInterval(timeId) if timeId
    DCore.Shutdown.quit()


confirm_ok = (power)->
    echo "--------------confirm_ok(#{power})-------------"
    switch power
        when "lock" then destory_all()
        when "suspend" then destory_all()
    power_force(power)
    clearInterval(timeId) if timeId

document.body.style.height = window.innerHeight
document.body.style.width = window.innerWidth
#background = new Background(APP_NAME)
#bg_url = background.get_current_user_background()
bg_url = DEFAULT_BG
document.body.style.backgroundImage = "url(#{bg_url})"

DEBUG_ANIMATION = false
if DEBUG_ANIMATION
    bg_el = create_img("bg_el","",document.body)
    try
        bg_el.src = bg_url
        bg_el.style.display = "block"
    catch e
        bg_el.style.display = "none"
        echo "#{e}"
else
    document.body.style.backgroundImage = "url(#{bg_url})"

TIME_SHOW = 500
showAnimation =(el,t)->
    if !DEBUG_ANIMATION
        document.body.style.display = "none"
        jQuery(document.body).fadeIn(t)
        
        #document.body.style.opacity = "0.0"
#        jQuery(document.body).animate(
            #{opacity:'1.0';},t
        #)
        return
    
    t_first = 300
    t_second = 200
    document.body.style.opacity = "0.0"
    bg_el.style.opacity = "0.0"
    el.style.opacity = "0.0"
    
    jQuery(document.body).animate(
        {opacity:'0.5';},t_first
    )
    jQuery(bg_el).animate(
        {opacity:'0.5';},
        t_first,
        "linear",=>
            jQuery(document.body).animate(
                {opacity:'1.0';},t_second
            )
            jQuery(bg_el).animate(
                {opacity:'1.0';},t_second
            )
            jQuery(el).animate(
                {opacity:'1.0';},t_second
            )
            echo "showAnimation end"
    )


#DCore.signal_connect("draw_background", (info)->
    #echo "draw_background:url(#{info.path})"
    #document.body.style.backgroundImage = "url(#{info.path})"
#)
