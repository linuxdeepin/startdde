/*
 * Copyright (C) 2016 ~ 2018 Deepin Technology Co., Ltd.
 *
 * Author:     ganjing <ganjing@uniontech.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

#include <math.h>
#include <glib.h>
#include <stdio.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <unistd.h>
#include <linux/input.h>

#include "sensor.h"

bool doCheck = false;
struct sensor_axis axis = {0};
int accel_offset[3] = {0};
int orientation = -1;

int open_device()
{
    int dev_fd = -1;
    dev_fd = open("/dev/mma8452_daemon", O_RDONLY);
    if (dev_fd < 0) {
        return -1;
    }
    return dev_fd;
}

int close_device(int fd)
{
    if (fd >= 0) {
        close(fd);
    }
    return 0;
}

int get_input()
{
    int fd = -1;
    unsigned i;
    static struct input_dev dev[255];

    const char *dirname = "/dev/input";
    char devname[PATH_MAX];
    char *filename;
    DIR *dir;
    struct dirent *de;

    for (i = 0; i < sizeof(dev) / sizeof(dev[0]); i++) {
        dev[i].fd = -1;
        dev[i].name[0] = '\0';
    }
    i = 0;

    dir = opendir(dirname);
    if (dir == NULL)
        return -1;

    strcpy(devname, dirname);
    filename = devname + strlen(devname);
    *filename++ = '/';
    while ((de = readdir(dir))) {
        if (de->d_name[0] == '.' &&
            (de->d_name[1] == '\0' || (de->d_name[1] == '.' && de->d_name[2] == '\0')))
                continue;

        strcpy(filename, de->d_name);
        fd = open(devname, O_RDONLY);
        if (fd >= 0) {
            char name[80];
            if (ioctl(fd, EVIOCGNAME(sizeof(name) - 1), &name) >= 1) {
                dev[i].fd = fd;
                strncpy(dev[i].name, name, sizeof(dev[i].name));
            }
        }
        i++;
    }
    closedir(dir);

    for (i = 0; i < sizeof(dev) / sizeof(dev[0]); i++) {
        if (!strncmp("gsensor", dev[i].name, sizeof(dev[i].name))) {
            fd = dev[i].fd;
            continue;
        }

        if (dev[i].fd > 0) {
            close(dev[i].fd);
        }
    }
    return fd;
}

void close_input(int fd)
{
    if (fd >= 0) {
        close(fd);
    }
}

void read_calibration(int fd)
{
    if (fd < 0) {
        return;
    }

    int result = ioctl(fd, GSENSOR_IOCTL_GET_CALIBRATION, &accel_offset);
    if (result < 0) {
        return;
    }
}

int start_device(int fd)
{
    if (fd < 0) {
        return -1;
    }

    int result = ioctl(fd, GSENSOR_IOCTL_START);
    if (result < 0) {
        return -1;
    }
    return result;
}

void read_events(int* fd)
{
    struct input_event event;
    while(1) {
        if (*fd < 0) {
            return;
        } else {
            if (read(*fd, &event, sizeof(struct input_event)) == sizeof(struct input_event)) {
                if (event.type == EV_ABS) {
                    process_event(event.code, event.value);
                    if (doCheck) {
                        value_changed(axis);
                        doCheck = false;
                    }
                }
            }
        }
    }
}

void process_event(int code, int value)
{
    switch (code) {
        case EVENT_TYPE_ACCEL_X: {
            axis.x = (value - accel_offset[0]) * ACCELERATION_RATIO_ANDROID_TO_HW;
            break;
        }
        case EVENT_TYPE_ACCEL_Y:
        {
            axis.y = (value - accel_offset[1]) * ACCELERATION_RATIO_ANDROID_TO_HW;
            break;
        }
        case EVENT_TYPE_ACCEL_Z:
        {
            axis.z = (value - accel_offset[2]) * ACCELERATION_RATIO_ANDROID_TO_HW;
            doCheck = true;
            break;
        }
    }
}

int orientation_calc(struct sensor_axis axis)
{
    float absx = abs(axis.x);
    float absy = abs(axis.y);
    float absz = abs(axis.z);
    if (absx > absy && absx > absz) {
        if (axis.x > OFFSET) {
            if (orientation != 0) {
                return 0;
            }
        } else if (axis.x < -OFFSET) {
            if (orientation != 1) {
                return 1;
            }
        }
    } else if (absy > absx && absy > absz) {
        if (axis.y > OFFSET) {
            if (orientation != 2) {
                return 2;
            }
        } else if (axis.y < -OFFSET) {
            if (orientation != 3) {
                return 3;
            }
        }
    } else if (absz > absx && absz > absy) {
        if (axis.z > 0) {
            return -1;
        } else {
            return -2;
        }
    } else {
        return -3;
    }
}

void value_changed(struct sensor_axis axis)
{
    // 陀螺仪厂商规定重力加速度为10, 因此x,y,z三个方向的重力加速度的矢量和应该是小于等于10的, 数据可能存在一定误差,这里取10.5,
    // 所以三个方向重力加速度的模的平方和应该是小于10.5的平方
    if (pow(axis.x, 2) + pow(axis.y, 2) + pow(axis.z, 2) > pow(10.5, 2)) {
        return;
    }

    int ret = orientation_calc(axis);
    if (ret >= 0) {
        char command[256];
        if (ret == 0 && orientation != ret) {
            orientation = ret;
            strcpy(command, "xrandr -o right");
            system(command);
        } else if (ret == 1 && orientation != ret) {
            orientation = ret;
            strcpy(command, "xrandr -o left");
            system(command);
        } else if (ret == 2 && orientation != ret) {
            orientation = ret;
            strcpy(command, "xrandr -o normal");
            system(command);
        } else if (ret == 3 && orientation != ret) {
            orientation = ret;
            strcpy(command, "xrandr -o inverted");
            system(command);
        }
    }
}

