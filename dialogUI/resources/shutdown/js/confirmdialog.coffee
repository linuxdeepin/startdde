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

class ConfirmDialog extends Widget
    timeId = null
    CANCEL = 0
    OK = 1
    choose_num = OK

    constructor: (opt)->
        super
        i = null
        @opt = opt
        for tmp,j in option
            if tmp is opt then i  = j
        if i is null
            echo "no this power option!"
            return
        if i < 2 or i > 4 then return
        @i = i
        powerchoose = null

    destory:->
        document.body.removeChild(@element)


    frame_build:->
        i = @i
        frame_confirm = create_element("div", "frame_confirm", @element)
        frame_confirm.addEventListener("click",->
            frame_click = true
        )
        
        left = create_element("div","left",frame_confirm)
        @img_confirm = create_img("img_confirm","img/normal/#{option[i]}.png",left)
        
        right = create_element("div","right",frame_confirm)
        @message_confirm = create_element("div","message_confirm",right)

        button_confirm = create_element("div","button_confirm",right)
        
        @button_cancel = create_element("div","button_cancel",button_confirm)
        @button_cancel.textContent = _("Cancel")

        @button_ok = create_element("div","button_ok",button_confirm)
        echo "----------------power_can check-------------"
        if power_can(@opt) then @style_for_direct()
        else @style_for_force()

        @button_cancel.addEventListener("click",->
            echo "button_cancel click"
            destory_all()
        )
        @button_ok.addEventListener("click",->
            echo "button_ok click"
            confirm_ok(option[i])
        )

        @button_cancel.addEventListener("mouseover",=>
            choose_num = CANCEL
            @hover_state(choose_num)
        )
        @button_cancel.addEventListener("mouseout",=>
            @normal_state(CANCEL)
        )
        @button_ok.addEventListener("mouseover",=>
            choose_num = OK
            @hover_state(choose_num)
        )

        @button_ok.addEventListener("mouseout",=>
            @normal_state(OK)
        )

        apply_animation(right,"show_confirm","0.3s")
        right.addEventListener("webkitAnimationEnd",=>
            right.style.opacity = "1.0"
        ,false)
    
    style_for_direct:->
        echo "style_for_direct:power_can true!"
        i = @i
        @img_confirm.src = "img/normal/#{option[i]}.png"
        @message_confirm.textContent = message_text[i].args(60)
        @button_ok.textContent = option_text[i]

    style_for_force:->
        echo "style_for_force:power_can false!"
        i = @i
        @img_confirm.src = "img/normal/#{option[i]}.png"
        @message_confirm.textContent = message_text[i].args(60)
        @button_ok.textContent = option_text_force[i]
        @button_ok.style.color = "rgba(255,128,114,1.0)"
    
    interval:(time)->
        i = @i
        that = @
        clearInterval(timeId) if timeId
        timeId = setInterval(->
            time--
            that.message_confirm.textContent = message_text[i].args(time)
            if time == 0
                clearInterval(timeId)
                if 2 <= i <= 4 then confirm_ok(option[i])
        ,1000)

    hover_state: (choose_num)->
        switch choose_num
            when OK
                @button_ok.style.color = "rgba(0,193,255,1.0)"
                @button_cancel.style.color = "rgba(255,255,255,0.5)"
            when CANCEL
                @button_cancel.style.color = "rgba(0,193,255,1.0)"
                @button_ok.style.color = "rgba(255,255,255,0.5)"
            else return

    normal_state: (choose_num)->
        switch choose_num
            when OK
                @button_ok.style.color = "rgba(255,255,255,0.5)"
                @button_cancel.style.color = "rgba(255,255,255,0.5)"
            when CANCEL
                @button_cancel.style.color = "rgba(255,255,255,0.5)"
                @button_ok.style.color = "rgba(255,255,255,0.5)"
            else return
    
    keydown:(keyCode)->
        change_choose =->
            if choose_num == OK then choose_num = CANCEL
            else choose_num = OK
            return choose_num

        choose_enter = =>
            i = @i
            switch choose_num
                when OK
                    if 2 <= i <= 4 then confirm_ok(option[i])
                when CANCEL
                    destory_all()
                else return

        #document.body.removeEventListener("keydown",arguments.callee,false)
        switch keyCode
            when LEFT_ARROW
                change_choose()
                @hover_state(choose_num)
            when RIGHT_ARROW
                change_choose()
                @hover_state(choose_num)
            when ENTER_KEY
                choose_enter()
            when ESC_KEY
                destory_all()
