#Copyright (c) 2011 ~ 2012 Deepin, Inc.
#              2011 ~ 2012 snyh
#
#Author:      snyh <snyh@snyh.org>
#Maintainer:  snyh <snyh@snyh.org>
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

class AudioPlay
    default_audio_player_name = null
    default_audio_player_icon = null
    Metadata = null
    launched_status = false

    constructor: ->
        try
            mpris = @get_mpris_dbus()
            if not mpris? then return
            echo mpris
            @mpris_dbus = DCore.DBus.session_object("#{mpris}", "/org/mpris/MediaPlayer2", "org.mpris.MediaPlayer2.Player")
            launched_status = true
        catch error
            launched_status = false
            echo "@mpris_dbus is null ,the player isnt launched!"

    
    get_mpris_dbus:->
        @mpris_dbus_min = "org.mpris.MediaPlayer2."
        dbus_all = []
        @mpris_dbus_all = []
        freedesktop_dbus = DCore.DBus.session_object("org.freedesktop.DBus","/","org.freedesktop.DBus")
        dbus_all = freedesktop_dbus.ListNames_sync()
        for dbus in dbus_all
            index = dbus.indexOf(@mpris_dbus_min)
            if index != -1
                name = dbus.substring(index + @mpris_dbus_min.length)
                @mpris_dbus_all.push({"mpris":dbus,"name":name})
        
        switch(@mpris_dbus_all.length)
            when 0 then return null
            when 1 then return @mpris_dbus_all[0].mpris
            else
                for dbus in @mpris_dbus_all
                    mpris = dbus.mpris
                    #if dbus.name is DCore.DEntry.get_default_audio_player_name().toLowerCase() return dbus.mpris
                    if dbus.name is "dmusic" then return dbus.mpris
                    return mpris


    get_launched_status:->
        return launched_status

    get_default_audio_player_name:->
        default_audio_player_name = DCore.DEntry.get_default_audio_player_name()

    get_default_audio_player_icon:->
        default_audio_player_icon = DCore.DEntry.get_default_audio_player_icon()

    getPlaybackStatus:->
        @mpris_dbus.PlaybackStatus

    Next:->
        @mpris_dbus.Next()

    Pause:->
        @mpris_dbus.Pause()

    Play:->
        @mpris_dbus.Play()

    PlayPause:->
        @mpris_dbus.PlayPause()

    Previous:->
        @mpris_dbus.Previous()

    Seek:->
        @mpris_dbus.Seek()

    SetPosition:->
        @mpris_dbus.SetPosition()

    getPosition:->
        @mpris_dbus.Position

    Stop:->
        @mpris_dbus.Stop()

    getVolume:->
        @mpris_dbus.Volume

    setVolume:(val)->
        if val > 1 then val = 1
        else if val < 0 then val = 0
        @mpris_dbus.Volume = val

    getMetadata:->
        Metadata = @mpris_dbus.Metadata

    getTitle:->
        @mpris_dbus.Metadata['xesam:title']

    getUrl:->
        #www url
        @mpris_dbus.Metadata['xesam:url']
    
    getalbum:->
        #zhuanji name
        @mpris_dbus.Metadata['xesam:album']

    getArtist:->
        #artist name
        @mpris_dbus.Metadata['xesam:artist']

    getArtUrl:->
        #artist img
        @mpris_dbus.Metadata['mpris:artUrl']
