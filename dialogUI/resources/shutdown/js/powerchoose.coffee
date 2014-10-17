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
        @opt = []
        @opt_img = []
        @opt_text = []
        @img_url_normal = []
        @img_url_hover = []
        @img_url_click = []
        @message_text = []
        document.body.appendChild(@element)
        @powercls = new Power()
        @powercls.power_get_inhibit()

        @accountscls = new Accounts("Shutdown")
        @username = @accountscls?.get_default_username()
        @userid = @accountscls?.get_user_id(@username)

    destory:->
        document.body.removeChild(@element)

    img_url_build:->
        for i of option
            @img_url_normal.push("img/#{option[i]}_normal.png")
            @img_url_hover.push("img/#{option[i]}_hover.png")
            @img_url_click.push("img/#{option[i]}_press.png")


    get_sessioned_on_power_dict : ->
        ["shutdown","restart"]

    get_sessioned_on_msg : ->
        @accountscls?.is_user_sessioned_on(@userid)
        @users_id_logined_len =  @accountscls?.users_id_logined.length
        users_id_logined = @accountscls?.users_id_logined
        if @users_id_logined_len > 1
            if @userid in users_id_logined
                for uid,i in users_id_logined
                    if @userid is uid
                        users_id_logined.splice(i,1)
            names = []
            for id in users_id_logined
                names.push(@accountscls.get_user_name(id))
            console.debug "current userid:#{@userid};other logged ids:(#{users_id_logined}) === names:(#{names})"
            if names.length > 1
                msg = _("The other users have logged in, shutdown or restart may cause the running data to be lost.")
            else if names.length == 1
                msg = _("User %1 has logged in, shutdown or restart may cause the running data to be lost.").args(names.toString())
            return msg
        else
            return null

    showMessage:(text)->
        if text is null or text is undefined
            return
        @message_div?.style.opacity = 1
        @message_text_div?.textContent = text


    setOptionDefault:(option_defalut)->
        #this key must get From system
        GetinFromKey = false
        for key,i in option
            if key is option_defalut
                if GetinFromKey
                    @select_state(i)
                else
                    choose_num = i
                    @hover_state(i)
                @opt[i].focus()

    message_div_build:->
        @message_div = create_element("div","message_div",@element)
        @message_img_div = create_element("div","message_img_div",@message_div)
        @message_img_div.style.backgroundImage = "url(img/waring.png)"
        @message_text_div = create_element("div","message_text_div",@message_div)
        @message_div.style.opacity = 0



    frame_build:->
        @img_url_build()

        sessioned_on_msg = @get_sessioned_on_msg()
        @message_div_build()
        button_div = create_element("div","button_div",@element)
        for key ,i in option
            @opt[i] = create_element("div","opt",button_div)
            @opt[i].style.backgroundColor = "rgba(255,255,255,0.0)"
            @opt[i].style.border = "1px solid rgba(255,255,255,0.0)"
            @opt[i].value = i
            @opt_img[i] = create_img("opt_img",@img_url_normal[i],@opt[i])
            @opt_img[i].alt = option_text[i]
            @opt_text[i] = create_element("div","opt_text",@opt[i])
            @opt_text[i].textContent = option_text[i]

            message_text = @powercls.inhibit_msg(key)
            if sessioned_on_msg? and (key in @get_sessioned_on_power_dict())
                message_text = sessioned_on_msg
            @message_text.push(message_text)

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
                power = option[i]
                if that.powercls.power_can(power)
                    that.opt_img[i].src = that.img_url_click[i]
                    that.fade(i)
            )
        @setOptionDefault("shutdown")
        @check_inhibit_message()
        showAnimation(@element,TIME_SHOW)

    css_inhibit:(i)->
        disable = !@powercls.power_can(option[i])
        if disable is true
            @opt[i].disable = "true"
            @opt_img[i].style.opacity = "0.3"
            @opt[i].style.cursor = "default"
        else
            @opt[i].disable = "false"
            @opt_img[i].style.opacity = "1.0"
            @opt[i].style.cursor = "pointer"

    check_inhibit_message: ->
        for bt,i in @opt
            @css_inhibit(i)
            @showMessage(@message_text[i])

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
        @showMessage(@message_text[i])
        for tmp,j in @opt_img
            if j == i and enable is true then tmp.src = @img_url_hover[i]
            else
                tmp.src = @img_url_normal[j]

    select_state:(i,enable = true)->
        select_state_confirm = true
        power = option[i]
        enable = @powercls.power_can(power)
        @showMessage(@message_text[i])
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
                #if @is_disable(option[choose_num]) then choose_num--
                #if choose_num == -1 then choose_num = @opt.length - 1
                @select_state(choose_num)
            when RIGHT_ARROW
                choose_num++
                if choose_num == option.length then choose_num = 0
                #if @is_disable(option[choose_num]) then choose_num++
                #if choose_num == @opt.length then choose_num = 0
                @select_state(choose_num)
            when ENTER_KEY
                i = choose_num
                @fade(i)
            when ESC_KEY
                destory_all()
