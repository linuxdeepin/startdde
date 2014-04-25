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

class PowerChoose extends Widget
    opt = []
    opt_img = []
    opt_text = []
    choose_num = -1
    select_state_confirm = false

    img_url_normal = []
    img_url_hover = []
    img_url_click = []
    
    constructor: ()->
        super
        confirmdialog = null
    
    setPos:->
        @element.style.display = "-webkit-box"
        echo "clientWidth:#{@element.clientWidth}"
        echo "clientHeight:#{@element.clientHeight}"
        w = @element.clientWidth
        h = @element.clientHeight
        w = 610 if w == 0
        h = 145 if h == 0
        left = (screen.width  - w) / 2
        bottom = (screen.height) / 2
        @element.style.left = "#{left}px"
        @element.style.bottom = "#{bottom}px"
    
    destory:->
        document.body.removeChild(@element)

    img_url_build:->
        for i of option
            img_url_normal.push("img/#{option[i]}_normal.png")
            img_url_hover.push("img/#{option[i]}_hover.png")
            img_url_click.push("img/#{option[i]}_press.png")


    showMessage:(text)->
        @message_div.style.display = "-webkit-box"
        @message_text_div.textContent = text

    frame_build:->
        @img_url_build()
        #frame = create_element("div", "frame", @element)
        @element.addEventListener("click",->
            frame_click = true
        )
        
        @message_div = create_element("div","message_div",@element)
        @message_img_div = create_element("div","message_img_div",@message_div)
        @message_img_div.style.backgroundImage = "url(img/waring.png)"
        @message_text_div = create_element("div","message_text_div",@message_div)
        @message_text_div.textContent = message_text["default"]
        @message_div.style.display = "none"

        button_div = create_element("div","button_div",@element)
       
        for tmp ,i in option
            opt[i] = create_element("div","opt",button_div)
            opt[i].style.backgroundColor = "rgba(255,255,255,0.0)"
            opt[i].style.border = "1px solid rgba(255,255,255,0.0)"
            opt[i].value = i
            opt_img[i] = create_img("opt_img",img_url_normal[i],opt[i])
            opt_text[i] = create_element("div","opt_text",opt[i])
            opt_text[i].textContent = option_text[i]
            
            #this key must get From system
            GetinFromKey = false
            if tmp is "shutdown"
                if GetinFromKey
                    @select_state(i)
                else
                    choose_num = i
                    @hover_state(i)
                opt[i].focus()
            
            that = @
            #hover
            opt[i].addEventListener("mouseover",->
                i = this.value
                #choose_num = i
                that.hover_state(i)
            )
            
            #normal
            opt[i].addEventListener("mouseout",->
                i = this.value
                opt_img[i].src = img_url_normal[i]
            )

            #click
            opt[i].addEventListener("mousedown",->
                i = this.value
                opt_img[i].src = img_url_click[i]
            )
            opt[i].addEventListener("click",->
                i = this.value
                frame_click = true
                opt_img[i].src = img_url_click[i]
                that.fade(i)
            )
        @setPos()
        @check_inhibit()
        showAnimation(@element,TIME_SHOW)
    
    timefunc:(i) ->
        @destory()
        confirmdialog = new ConfirmDialog(option[i])
        confirmdialog.frame_build()
        document.body.appendChild(confirmdialog.element)
        confirmdialog.interval(60)
    
    switchToConfirmDialog:(i)->
        opt[i].style.backgroundColor = "rgba(255,255,255,0.0)"
        opt[i].style.border = "1px solid rgba(255,255,255,0.0)"
        opt[i].style.borderRadius = null
        time = 0.5
        for el,j in opt
            apply_animation(el,"fade_animation#{j}","#{time}s")
        opt[i].addEventListener("webkitAnimationEnd",=>
            @timefunc(i)
        ,false)
 

    css_inhibit:(i,enable = true)->
        if enable is true
            opt[i].disable = "true"
            opt[i].disable = "disable"
            opt[i].style.opacity = "0.3"
            inhibit = power_get_inhibit(option[i])
            if enable is false then @showMessage(inhibit?[2])
        else
            opt[i].disable = "false"
            opt[i].style.opacity = "1.0"

    check_inhibit: ->
        for bt,i in opt
            @css_inhibit(i,power_can(option[i]))

    fade:(i)->
        echo "--------------fade:#{option[i]}---------------"
        if power_can(option[i])
            echo "power_can true ,power_force"
            confirm_ok(option[i])
            #power_force(option[i])
        else
            echo "power_can false ,switchToConfirmDialog"
            @switchToConfirmDialog(i)

    hover_state:(i,enable = true)->
        #choose_num = i
        if select_state_confirm then @select_state(i)
        power = option[i]
        enable = power_can(power)
        inhibit = power_get_inhibit(power)
        if enable is false then @showMessage(inhibit?[2])
        for tmp,j in opt_img
            if j == i and enable is true then tmp.src = img_url_hover[i]
            else
                tmp.src = img_url_normal[j]
   
    select_state:(i,enable = true)->
        select_state_confirm = true
        power = option[i]
        enable = power_can(power)
        inhibit = power_get_inhibit(power)
        if enable is false then @showMessage(inhibit?[2])
        choose_num = i
        for tmp,j in opt
            if j == i and enable is true
                tmp.style.backgroundColor = "rgba(255,255,255,0.1)"
                tmp.style.border = "1px solid rgba(255,255,255,0.15)"
                tmp.style.borderRadius = "4px"
            else
                tmp.style.backgroundColor = "rgba(255,255,255,0.0)"
                tmp.style.border = "1px solid rgba(255,255,255,0.0)"
                tmp.style.borderRadius = null
 
    keydown:(keyCode)->
        switch keyCode
            when LEFT_ARROW
                choose_num--
                if choose_num == -1 then choose_num = option.length - 1
                @select_state(choose_num)
            when RIGHT_ARROW
                choose_num++
                if choose_num == option.length then choose_num = 0
                @select_state(choose_num)
            when ENTER_KEY
                i = choose_num
                @fade(i)
            when ESC_KEY
                destory_all()

