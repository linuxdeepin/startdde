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

    choose_num = -1
    select_state_confirm = false

    constructor: ()->
        super
        confirmdialog = null
        @opt = []
        @opt_img = []
        @opt_text = []
        @img_url_normal = []
        @img_url_hover = []
        @img_url_click = []
        @powercls = new Power()
        @powercls.power_get_inhibit()

    destory:->
        document.body.removeChild(@element)

    img_url_build:->
        for i of option
            @img_url_normal.push("img/#{option[i]}_normal.png")
            @img_url_hover.push("img/#{option[i]}_hover.png")
            @img_url_click.push("img/#{option[i]}_press.png")


    showMessage:(text)->
        @message_div.style.display = "-webkit-box"
        @message_text_div.textContent = text


    setOptionDefault:(option_defalut)->
        #this key must get From system
        GetinFromKey = false
        for tmp,i in option
            if tmp is option_defalut
                if GetinFromKey
                    @select_state(i)
                else
                    choose_num = i
                    @hover_state(i)
                @opt[i].focus()


    frame_build:->
        @img_url_build()
        @element.addEventListener("click",->
            frame_click = true
        )
        @message_div = create_element("div","message_div",@element)
        @message_img_div = create_element("div","message_img_div",@message_div)
        @message_img_div.style.backgroundImage = "url(img/waring.png)"
        @message_text_div = create_element("div","message_text_div",@message_div)
        @message_div.style.display = "none"

        button_div = create_element("div","button_div",@element)
        for tmp ,i in option
            @opt[i] = create_element("div","opt",button_div)
            @opt[i].style.backgroundColor = "rgba(255,255,255,0.0)"
            @opt[i].style.border = "1px solid rgba(255,255,255,0.0)"
            @opt[i].value = i
            @opt_img[i] = create_img("opt_img",@img_url_normal[i],@opt[i])
            @opt_img[i].alt = option_text[i]
            @opt_text[i] = create_element("div","opt_text",@opt[i])
            @opt_text[i].textContent = option_text[i]
            that = @
            #hover
            @opt[i].addEventListener("mouseover",->
                i = this.value
                that.hover_state(i)
            )
            #normal
            @opt[i].addEventListener("mouseout",->
                i = this.value
                that.opt_img[i].src = that.img_url_normal[i]
            )
            #click
            @opt[i].addEventListener("click", (e)->
                e.preventDefault()
                e.stopPropagation()
                i = this.value
                frame_click = true
                power = option[i]
                if that.powercls.power_can(power)
                    that.opt_img[i].src = that.img_url_click[i]
                    that.fade(i)
            )
        @setOptionDefault("shutdown")
        @check_inhibit()
        showAnimation(@element,TIME_SHOW)

    css_inhibit:(i)->
        disable = !@powercls.power_can(option[i])
        if disable is true
            @opt[i].disable = "true"
            @opt_img[i].style.opacity = "0.3"
            @opt[i].style.cursor = "default"
            @showMessage(@powercls.inhibit_msg(option[i]))
        else
            @opt[i].disable = "false"
            @opt_img[i].style.opacity = "1.0"
            @opt[i].style.cursor = "pointer"

    check_inhibit: ->
        for bt,i in @opt
            @css_inhibit(i)

    fade:(i)->
        echo "--------------fade:#{option[i]}---------------"
        if @powercls.power_can(option[i])
            @confirm_ok(option[i])

    confirm_ok : (power)->
        echo "--------------confirm_ok(#{power})-------------"
        destory_all()
        switch power
            when "user_switch"
                DCore.Shutdown.switch_to_greeter()
            else
                @powercls.power_force_session(power)

    hover_state:(i,enable = true)->
        #choose_num = i
        if select_state_confirm then @select_state(i)
        power = option[i]
        enable = @powercls.power_can(power)
        if enable is false then @showMessage(@powercls.inhibit_msg(power))
        for tmp,j in @opt_img
            if j == i and enable is true then tmp.src = @img_url_hover[i]
            else
                tmp.src = @img_url_normal[j]

    select_state:(i,enable = true)->
        select_state_confirm = true
        power = option[i]
        enable = @powercls.power_can(power)
        if enable is false then @showMessage(@powercls.inhibit_msg(power))
        choose_num = i
        for tmp,j in @opt
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
                #if @is_disable(@option[choose_num]) then choose_num--
                #if choose_num == -1 then choose_num = @opt.length - 1
                @select_state(choose_num)
            when RIGHT_ARROW
                choose_num++
                if choose_num == option.length then choose_num = 0
                #if @is_disable(@option[choose_num]) then choose_num++
                #if choose_num == @opt.length then choose_num = 0
                @select_state(choose_num)
            when ENTER_KEY
                i = choose_num
                @fade(i)
            when ESC_KEY
                destory_all()

