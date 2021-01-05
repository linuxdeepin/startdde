package display

import (
	"fmt"
	"math"

	x "github.com/linuxdeepin/go-x11-client"
)

const INT_MAX = int(^uint(0) >> 1)

const (
	rectPoint0 int = iota
	rectPoint1
	rectPoint2
	rectPoint3
)

type Point struct {
	X int
	Y int
}

func (m *Manager) intersects(m0, m1 x.Rectangle) bool {
	mid0X := (m0.X + m0.X + int16(m0.Width)) / 2
	mid0Y := (m0.Y + m0.Y + int16(m0.Height)) / 2
	mid1X := (m1.X + m1.X + int16(m1.Width)) / 2
	mid1Y := (m1.Y + m1.Y + int16(m1.Height)) / 2

	aX := math.Abs(float64(mid0X - mid1X))
	aY := math.Abs(float64(mid0Y - mid1Y))

	maX := math.Abs(float64((m0.Width + m1.Width) / 2))
	maY := math.Abs(float64((m0.Height + m1.Height) / 2))

	if aX < maX && aY < maY {
		return true
	}

	return false
}

func (m *Manager) bestMoveOffset(r0, r1 x.Rectangle) (int, int, error) {
	selfTopLeft := Point{X: int(r0.X), Y: int(r0.Y)}
	selfPoints := make([]Point, 0)
	selfPoints = append(selfPoints, selfTopLeft, Point{int(r0.X) + int(r0.Width), int(r0.Y)}, Point{int(r0.X), int(r0.Y) + int(r0.Height)}, Point{int(r0.X) + int(r0.Width), int(r0.Y) + int(r0.Height)})

	otherTopLeft := Point{X: int(r1.X), Y: int(r1.Y)}
	otherPoints := make([]Point, 0)
	otherPoints = append(otherPoints, otherTopLeft, Point{int(r1.X) + int(r1.Width), int(r1.Y)}, Point{int(r1.X), int(r1.Y) + int(r1.Height)}, Point{int(r1.X) + int(r1.Width), int(r1.Y) + int(r1.Height)})

	cb := func(x, y, target1, target2 int) bool {
		if (x == target1 && y == target2) || (x == target2 && y == target1) {
			return true
		}
		return false
	}

	var bestOffset Point
	needOffset := false
	min := INT_MAX
	for i, p1 := range selfPoints {
		for j, p2 := range otherPoints {
			offset := Point{
				X: p1.X - p2.X,
				Y: p1.Y - p2.Y,
			}
			if m.mutiMonitorsPos == MonitorsLeftRight {
				if !cb(i, j, rectPoint0, rectPoint1) && !cb(i, j, rectPoint2, rectPoint3) {
					continue
				}
			} else if m.mutiMonitorsPos == MonitorsUpDown {
				if !cb(i, j, rectPoint0, rectPoint2) && !cb(i, j, rectPoint1, rectPoint3) {
					continue
				}
			} else if m.mutiMonitorsPos == MonitorsDiagonal {
				if !cb(i, j, rectPoint1, rectPoint2) && !cb(i, j, rectPoint0, rectPoint3) {
					continue
				}
			} else {

			}

			mt := int(math.Pow(float64(offset.X), 2) + math.Pow(float64(offset.Y), 2))
			if mt >= min {
				continue
			}

			// test intersect
			rTemp := x.Rectangle{X: r0.X - int16(offset.X), Y: r0.Y - int16(offset.Y), Width: r0.Width, Height: r0.Height}
			if m.intersects(rTemp, r1) {
				continue
			}

			min = mt
			bestOffset = offset
			needOffset = true
		}
	}

	if !needOffset {
		return 0, 0, fmt.Errorf("no offset")
	}

	return bestOffset.X, bestOffset.Y, nil
}

func (m *Manager) bestMovePosition(r0, r1 x.Rectangle) (x.Rectangle, x.Rectangle, error) {
	logger.Debug("bestMovePosition before move primary monitor:", r0)
	logger.Debug("bestMovePositionr before move second monitor:", r1)
	offsetX, offsetY, err := m.bestMoveOffset(r0, r1)
	if err != nil {
		return x.Rectangle{}, x.Rectangle{}, err
	}

	//将r0和r1进行拼接，拼接后为rt0和rt1
	rt0 := x.Rectangle{
		X:      r0.X - int16(offsetX),
		Y:      r0.Y - int16(offsetY),
		Width:  r0.Width,
		Height: r0.Height,
	}

	rt1 := x.Rectangle{
		X:      r1.X,
		Y:      r1.Y,
		Width:  r1.Width,
		Height: r1.Height,
	}

	//计算合并后的左上角坐标
	minX := int(math.Min(float64(rt0.X), float64(rt1.X)))
	minY := int(math.Min(float64(rt0.Y), float64(rt1.Y)))

	//对两个矩形进行左上角坐标偏移计算
	rt0.X = rt0.X - int16(minX)
	rt0.Y = rt0.Y - int16(minY)

	rt1.X = rt1.X - int16(minX)
	rt1.Y = rt1.Y - int16(minY)

	logger.Debug("bestMovePosition after move primary monitor:", rt0)
	logger.Debug("bestMovePositionr after move second monitor:", rt1)

	return rt0, rt1, nil
}
