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

#ifndef __SENSOR_H__
#define __SENSOR_H__

#include <linux/ioctl.h>

#define ACCELERATION_RATIO_ANDROID_TO_HW        (9.80665f / 16384)
#define GSENSOR_IOCTL_MAGIC 'a'
#define GSENSOR_IOCTL_CLOSE                _IO(GSENSOR_IOCTL_MAGIC, 0x02)
#define GSENSOR_IOCTL_START                _IO(GSENSOR_IOCTL_MAGIC, 0x03)
#define GSENSOR_IOCTL_GET_CALIBRATION      _IOR(GSENSOR_IOCTL_MAGIC, 0x11, int[3])

#define EVENT_TYPE_ACCEL_X          ABS_X
#define EVENT_TYPE_ACCEL_Y          ABS_Y
#define EVENT_TYPE_ACCEL_Z          ABS_Z

#define RADIANS_TO_DEGREES 180.0/M_PI
#define OFFSET 5

struct input_event;

struct input_dev {
    int fd;
    char name[80];
};

struct sensor_axis {
    float x;
    float y;
    float z;
};

int open_device();
int close_device(int fd);
int start_device(int fd);
int stop_device(int fd);
int get_input();
void read_calibration(int fd);
void read_events(int fd);
void process_event(int code, int value);
int orientation_calc(struct sensor_axis axis);
void value_changed(struct sensor_axis axis);

#endif
