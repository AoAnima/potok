package main

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	. "aoanima.ru/Logger"

	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/ui/wm"
	"github.com/rodrigocfd/windigo/win"
	"github.com/rodrigocfd/windigo/win/co"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	Kernel32 = syscall.NewLazyDLL("Kernel32.dll")

	ПолучитьСостояниеКлавиатуры = user32.NewProc("GetKeyboardState")
	кодВСимвол                  = user32.NewProc("ToUnicodeEx")
	СледующийХук                = user32.NewProc("CallNextHookEx")
	SetWindowLong               = user32.NewProc("SetWindowLongW")
	// SetLayeredWindowAttributes      = user32.NewProc("SetLayeredWindowAttributes")
	ReleaseCapture    = user32.NewProc("ReleaseCapture")
	SetTextColor      = gdi32.NewProc("SetTextColor")
	GetLastError      = Kernel32.NewProc("GetLastError")
	GetCaretPos       = user32.NewProc("GetCaretPos")
	SendInput         = user32.NewProc("SendInput")
	AttachThreadInput = user32.NewProc("AttachThreadInput")
)

type ДКУ win.HDC // ДескрипторКонтекстаУстройства
type ДО win.HWND // ДескрипторОкна

type Версия struct {
	релиз  int
	мажор  int
	минор  int
	сборка int
}
type ОписаниеПрограммы struct {
	Имя    string
	Версия Версия
}

var Поток = ОписаниеПрограммы{
	Имя: "Поток",
	Версия: Версия{
		релиз:  1,
		мажор:  0,
		минор:  0,
		сборка: 0,
	},
}

var УказательНаПоток = uintptr(unsafe.Pointer(&Поток))

type СтруктураКлавиатурногоХука struct {
	ВиртуальныйКод           ВиртуальныйКод
	СканКод                  СканКод
	Флаги                    uint32
	Время                    uint32
	ДополнительнаяИнформация uintptr
}

type ДанныеКлавиатурногоСобытия struct {
	СтруктураКлавиатурногоХука
	ТипСобытия win.WPARAM
}

type ВиртуальныйКод uint32
type СканКод uint32

type Кнопка struct {
	код        ВиртуальныйКод
	строкаКода string
	буквы      map[string][]string
}

type ПраймОкно struct {
	окно    ui.WindowMain
	надпись ui.Static
	// кнопки          []ui.Button
	статик          map[ВиртуальныйКод]ui.Static
	состояниеКнопок map[ВиртуальныйКод]bool // Добавляем поле для хранения состояния кнопок
	нажатаяКнопка   ВиртуальныйКод
	сетка           Сетка
}

type ОкноПодсказок struct {
	окно            ui.WindowMain
	надпись         ui.Static
	статик          map[string]ui.Static
	состояниеКнопок map[ВиртуальныйКод]bool // Добавляем поле для хранения состояния кнопок
	сетка           Сетка
}

type INPUT struct {
	Type uint32
	Ki   KEYBDINPUT
}

// type INPUT struct {
// 	Type uint32
// 	Data []byte // Размер структуры INPUT на 64-битной системе
// }

type KEYBDINPUT struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
	Unused      [8]byte
}

const (
	INPUT_KEYBOARD        = 1
	KEYEVENTF_EXTENDEDKEY = 0x0001
	KEYEVENTF_KEYUP       = 0x0002
	KEYEVENTF_UNICODE     = 0x0004
	KEYEVENTF_SCANCODE    = 0x0008
	VK_LWIN               = 0x5B // Виртуальный код клавиши "Windows"

)

var Алвафит = map[ВиртуальныйКод]Кнопка{

	0x51: {0x51, "0x51", map[string][]string{"en": []string{"E", "T"}, "ру": []string{"И", "Б", "Ы"}}},
	0x57: {0x57, "0x57", map[string][]string{"en": []string{"A", "O"}, "ру": []string{"В", "Ь", "Ъ"}}},
	0x45: {0x45, "0x45", map[string][]string{"en": []string{"I", "N"}, "ру": []string{"Д", "Е", "Ё"}}},
	0x52: {0x52, "0x52", map[string][]string{"en": []string{"S", "H"}, "ру": []string{"Ж", "З", "Н"}}},

	// Второй ряд (4 кнопки)
	0x41: {0x41, "0x41", map[string][]string{"en": []string{"R", "D"}, "ру": []string{"A", "Й"}}},
	0x53: {0x53, "0x53", map[string][]string{"en": []string{"L", "C"}, "ру": []string{"К", "Л"}}},
	0x44: {0x44, "0x44", map[string][]string{"en": []string{"U", "M"}, "ру": []string{"М", "П"}}},
	0x46: {0x46, "0x46", map[string][]string{"en": []string{"W", "F"}, "ру": []string{"О", "Р"}}},

	// Третий ряд (4 кнопки)
	0x5A: {0x5A, "0x5A", map[string][]string{"en": []string{"G", "Y"}, "ру": []string{"Ф", "Х", "Э", "Ю"}}},
	0x58: {0x58, "0x58", map[string][]string{"en": []string{"P", "B"}, "ру": []string{"Ц", "Ч", "Ш", "Щ"}}},
	0x43: {0x43, "0x43", map[string][]string{"en": []string{"V", "K"}, "ру": []string{"Р", "С"}}},
	0x56: {0x56, "0x56", map[string][]string{"en": []string{"J", "X", "Q", "Z"}, "ру": []string{"Т", "У"}}},
}

var Клавиатура = []ВиртуальныйКод{
	0x51,
	0x57,
	0x45,
	0x52,

	0x41,
	0x53,
	0x44,
	0x46,

	0x5A,
	0x58,
	0x43,
	0x56,
}

// Канал для обновления UI
var каналОбновленияОкна = make(chan ДанныеКлавиатурногоСобытия, 100)
var ОсновноеОкноПрограммы *ПраймОкно

func main() {
	runtime.LockOSThread()
	ХукКлавиатуры()
	ОсновноеОкноПрограммы = НовоеОкно()

	// Горутина для обновления UI
	go ПотокОбновленияЮИ()

	go func() {
		НовоеОкноПодсказок()

		дескриптор := ОсновноеОкноПодсказок.окно.RunAsMain()
		Инфо("дескриптор %+v \n", дескриптор)

	}()

	ОсновноеОкноПрограммы.окно.RunAsMain()
	log.Println(" Окно ")

	close(каналОбновленияОкна)
}

var зажатаяКлавиша = make(map[ВиртуальныйКод]bool)

func ХукКлавиатуры() {

	win.SetWindowsHookEx(co.WH_KEYBOARD_LL, func(код int32, типСобытия win.WPARAM, структураКлавишы win.LPARAM) uintptr {
		if код >= 0 {
			// if типСобытия == win.WPARAM(co.WM_KEYDOWN) {

			структураКлавиатуры := (*СтруктураКлавиатурногоХука)(unsafe.Pointer(структураКлавишы))

			каналОбновленияОкна <- ДанныеКлавиатурногоСобытия{
				*структураКлавиатуры,
				типСобытия,
			}
			// Инфо(" структураКлавиатуры %+v  типСобытия %+v \n", структураКлавиатуры, типСобытия)

			if структураКлавиатуры.ДополнительнаяИнформация == УказательНаПоток { // пока == , тоесть обрабатываем все собатиыя, нужно заменить на != чтобы передавались только события программы
				// если в дополнительнойинформации событие данных о том что событие было сгенерировано програмое которе равно УказательНаПоток то вывод на экран символа не долэен производится иначе если событие сгенерировано программой и имеет УказательНаПоток то выводим на экран

				Инфо(" структураКлавиатуры.ДополнительнаяИнформация %+v \n", структураКлавиатуры.ДополнительнаяИнформация)
				// ret, _, _ := СледующийХук.Call(0, uintptr(code), uintptr(wp), uintptr(lp))
				return 1
			}
			// }
		}

		ret, _, _ := СледующийХук.Call(0, uintptr(код), uintptr(типСобытия), uintptr(структураКлавишы))

		return ret
		// return 1
	}, 0, 0)

}

func ПотокОбновленияЮИ() {
	for СобытиеКлавиатуры := range каналОбновленияОкна {

		func(СобытиеКлавиатуры ДанныеКлавиатурногоСобытия) {

			if СобытиеКлавиатуры.ТипСобытия == win.WPARAM(co.WM_KEYDOWN) {
				runtime.LockOSThread()

				// if колВо, буква := ВЮникод(&СобытиеКлавиатуры); колВо > 0 {

					// Инфо("буква [2]uint16 %+v \n", буква)

					// буква := string(utf16.Decode(буква[:колВо]))
					ОсновноеОкноПодсказок.надпись.SetText(fmt.Sprintf("ВиртуальныйКод %v буква %s", СобытиеКлавиатуры.ВиртуальныйКод))
					// ОсновноеОкноПодсказок.надпись.SetText(fmt.Sprintf("ВиртуальныйКод %v буква %s", СобытиеКлавиатуры.ВиртуальныйКод, буква))

					ОсновноеОкноПрограммы.надпись.SetText(fmt.Sprintf("Код клавиши: 0x%X", СобытиеКлавиатуры.ВиртуальныйКод))
					НакопительКодов(&СобытиеКлавиатуры)
					// ПечатьТекста(буква)

				// } else {
				// 	Инфо("Код клавиши: 0x%X\n", СобытиеКлавиатуры.ВиртуальныйКод)
				// 	ОсновноеОкноПрограммы.надпись.SetText(fmt.Sprintf("Код клавиши: 0x%X", СобытиеКлавиатуры.ВиртуальныйКод))
				// 	// каналОбновленияОкна <- fmt.Sprintf("Код клавиши: 0x%X", vkCode)
				// }

				// делаем цвет кнопки светлее а через 1 секунду востанавливаем
				МиганиеКнопки(&СобытиеКлавиатуры)

				runtime.UnlockOSThread()
			}
		}(СобытиеКлавиатуры)
	}
}

func ВЮникод(СобытиеКлавиатуры *ДанныеКлавиатурногоСобытия) (uintptr, [2]uint16) {
	var состояниеКлавишы [256]byte

	ПолучитьСостояниеКлавиатуры.Call(uintptr(unsafe.Pointer(&состояниеКлавишы[0])))
	Инфо(" состояниеКлавишы %+v  %+v \n", состояниеКлавишы, СобытиеКлавиатуры.ТипСобытия)
	var буква [2]uint16
	Количество, _, _ := кодВСимвол.Call(
		uintptr(СобытиеКлавиатуры.ВиртуальныйКод),
		uintptr(СобытиеКлавиатуры.СканКод),
		uintptr(unsafe.Pointer(&состояниеКлавишы[0])),
		uintptr(unsafe.Pointer(&буква[0])),
		2,
		0,
		0)
	Инфо("Количество %+v буква %+v \n", Количество, буква)

	return Количество, буква
}

type БуферКодов struct {
	ТекущийНабор []ВиртуальныйКод
	ВсеНаборы    [][]ВиртуальныйКод
}

/*
НакопительКодов функция созраняем все коды нажатых клавиь в массив или буфер, если нажат пробел или шифт то происходит подбор слова который соттветсвует нажатым кавишам, и вывод его на экран
*/
func НакопительКодов(СобатиыеКлавиатуры *ДанныеКлавиатурногоСобытия) {

	// ПечатьТекста(слово)
}

func ПечатьБуквы(code ВиртуальныйКод, press bool) {
	Инфо(" ПечатьБуквы %+v press %+v \n", code, press)

	var direction uint32
	if !press {
		direction = 2
	}
	direction = 2
	inputs := INPUT{
		Type: INPUT_KEYBOARD,
		Ki: KEYBDINPUT{
			//WVk:         uint16(руна),
			WScan:       uint16(code),
			DwFlags:     direction,
			DwExtraInfo: УказательНаПоток,
		}}

	рез, рез2, ош := SendInput.Call(1,
		uintptr(unsafe.Pointer(&inputs)),
		uintptr(int32(unsafe.Sizeof(inputs))), // Важно: передаем размер INPUT, а не inputs[0]
	)

	Инфо("ПечатьБуквы  рез %+v, рез2 %+v, ош %+v \n", рез, рез2, ош)

}

func ПечатьТекста(текст string) {

	// активноеОкно, _ := ПолучитьАктивноеОкноИКаретку()
	// Инфо("ПечатьБуквы %v", активноеОкно)ыsrdыкв  dfgrewssaфdf

	// if !активноеОкно.IsWindow() || !активноеОкно.IsWindowVisible() {
	// 	return
	// }

	букваДляВывода := utf16.Encode([]rune(текст))
	// Инфо("ПечатьТекста букваДляВывода %+v \n", букваДляВывода)

	inputs := make([]INPUT, 0, len(букваДляВывода))

	for _, руна := range букваДляВывода {
		// Инфо("руна %v", руна)

		// Заполняем структуру правильно
		inputs = append(inputs, INPUT{
			Type: INPUT_KEYBOARD,
			Ki: KEYBDINPUT{
				//WVk:         uint16(руна),
				WScan:   uint16(руна),
				DwFlags: KEYEVENTF_UNICODE,

				DwExtraInfo: УказательНаПоток,
			}})
		// Фокусируем окно перед вводом
		// установленоАктивноеОкно := активноеОкно.SetForegroundWindow()
		// Инфо("установленоАктивноеОкно %v", установленоАктивноеОкно)

		// Нажатие клавиши ds

	}
	рез, _, ош := SendInput.Call(
		uintptr(uint32(len(inputs))),
		uintptr(unsafe.Pointer(&inputs[0])),
		uintptr(int32(unsafe.Sizeof(inputs[0]))), // Важно: передаем размер INPUT, а не inputs[0]
	)

	if рез == 0 {
		errorCode, e, lastcode := GetLastError.Call()
		ВыводОшибки("Ошибка SendInput (нажатие): код=%v e= %v lastcode=%v", errorCode, e, lastcode.Error())
		ВыводОшибки("Ошибка ош=%v", ош.Error())

		Инфо("Параметры SendInput:")
		Инфо("  Количество inputs: %d", len(inputs))
		Инфо("  Указатель на inputs: %v", unsafe.Pointer(&inputs))
		Инфо("  Размер структуры INPUT: %d", unsafe.Sizeof(INPUT{}))
		Инфо("  Размер inputs: %d", unsafe.Sizeof(inputs))
		Инфо("  Type: %d", inputs[0].Type)
		Инфо("  WScan: %d", inputs[0].Ki.WScan)
		Инфо("  DwFlags: %d", inputs[0].Ki.DwFlags)

	}
}
func МиганиеКнопки(структураКлавиатурногоХука *ДанныеКлавиатурногоСобытия) {
	if статикКнопка, ок := ОсновноеОкноПрограммы.статик[структураКлавиатурногоХука.ВиртуальныйКод]; ок {

		ОсновноеОкноПрограммы.нажатаяКнопка = структураКлавиатурногоХука.ВиртуальныйКод
		// ОсновноеОкноПрограммы.состояниеКнопок[структураКлавиатурногоХука.ВиртуальныйКод] = true
		ДО := статикКнопка.Hwnd()
		// Инфо("МиганиеКнопки ДО 1 %+v \n", ДО)

		ДО.InvalidateRect(nil, true)
		// ОсновноеОкноПрограммы.сетка.Разместить()
		// Через 1 секунду возвращаем исходные цвета
		time.AfterFunc(200*time.Millisecond, func() {
			ОсновноеОкноПрограммы.нажатаяКнопка = 0
			// Инфо("МиганиеКнопки ДО 2 %+v \n", ДО)
			// ОсновноеОкноПрограммы.состояниеКнопок[структураКлавиатурногоХука.ВиртуальныйКод] = false
			ДО.InvalidateRect(nil, true)
		})

	}
}
func ТекстовыйБлок(родительскийКОнтейнер ui.AnyParent, текст string) ui.Static {
	return ui.NewStatic(
		родительскийКОнтейнер,
		ui.StaticOpts().
			Position(win.POINT{X: 10, Y: 10}).
			Text(текст),
		//Size(win.SIZE{Cx: 290, Cy: 30}),
	)
}

func СобытиеПеретаскивание(окно ui.WindowMain) {

	окно.On().WmLButtonDown(func(p wm.Mouse) {
		// Преобразуем координаты клиентской области в экранные
		позиция := win.POINT{X: p.Pos().X, Y: p.Pos().Y}
		окно.Hwnd().ClientToScreenPt(&позиция)

		// Отправляем сообщение системе, что было нажатие на заголовок окна
		окно.Hwnd().SendMessage(
			co.WM_NCLBUTTONDOWN,
			win.WPARAM(co.HT_CAPTION),
			win.LPARAM(win.MAKELONG(uint16(позиция.X), uint16(позиция.Y))),
		)

	})
}

func (окно ПраймОкно) ПриОтображении() {

	окно.окно.On().WmShowWindow(func(p wm.ShowWindow) {
		ДО := окно.окно.Hwnd()
		fmt.Printf("ДО: %v\n", ДО)
		ДО.SetLayeredWindowAttributes(0, 190, 0x00000002)
		ОсновноеОкноПрограммы.сетка.Разместить()
		// ПозицияОкна(ДО, 0, 0)

	})

}
func ПозицияОкна(ДО win.HWND, x int32, y int32) {

	ширинаЭкрана := int32(win.GetSystemMetrics(co.SM_CXSCREEN))
	высотаЭкрана := int32(win.GetSystemMetrics(co.SM_CYSCREEN))

	Инфо("ДО.GetWindowRect() %+v \n", ДО.GetWindowRect())
	Инфо("высотаЭкрана %+v  ширинаЭкрана %+v \n", высотаЭкрана, ширинаЭкрана)
	положение := int32(-1)

	ДО.SetWindowPos(win.HWND(uintptr(положение)), 50, высотаЭкрана-350, 300, 300, co.SWP_SHOWWINDOW|co.SWP_NOSIZE|co.SWP_ASYNCWINDOWPOS)

}

func (окно ПраймОкно) ИзменениеЦветаКнопок() {

	окно.окно.On().WmCtlColorStatic(func(p wm.CtlColor) win.HBRUSH {
		ДКУ := p.Hdc()
		кисть := win.CreateSolidBrush(win.RGB(29, 13, 41))
		// Устанавливаем цвет фона на фиолетовый

		ДО := p.HwndControl()

		if ОсновноеОкноПрограммы.нажатаяКнопка != 0 {
			// Инфо("ИзменениеЦветаКнопок ДО: %v  == окно.статик[ОсновноеОкноПрограммы.нажатаяКнопка].Hwnd() %v %v\n", ДО, окно.статик[ОсновноеОкноПрограммы.нажатаяКнопка].Hwnd(), ДО == окно.статик[ОсновноеОкноПрограммы.нажатаяКнопка].Hwnd())
		}

		// Начало добавления

		// for код, статикКнопка := range окно.статик {
		// if статикКнопка.Hwnd() == ДО {s
		if ОсновноеОкноПрограммы.нажатаяКнопка != 0 && ДО == окно.статик[ОсновноеОкноПрограммы.нажатаяКнопка].Hwnd() {
			ДКУ.SetBkColor(win.RGB(255, 0, 123))
			SetTextColor.Call(uintptr(ДКУ), uintptr(win.RGB(139, 234, 0)))
			кисть = win.CreateSolidBrush(win.RGB(255, 0, 123))
		} else {
			ДКУ.SetBkColor(win.RGB(29, 13, 41))
			// ДКУ.SetBkMode(co.BKMODE_OPAQUE)
			SetTextColor.Call(uintptr(ДКУ), uintptr(win.RGB(255, 255, 255)))
			// Возвращаем дескриптор кисти, если необходимо
		}

		// if окно.состояниеКнопок[код] {
		// 	ДКУ.SetBkColor(win.RGB(255, 0, 123))
		// 	SetTextColor.Call(uintptr(ДКУ), uintptr(win.RGB(139, 234, 0)))
		// 	кисть = win.CreateSolidBrush(win.RGB(255, 0, 123))
		// } else {
		// 	ДКУ.SetBkColor(win.RGB(29, 13, 41))
		// 	// Фиолетовый цвет
		// 	SetTextColor.Call(uintptr(ДКУ), uintptr(win.RGB(255, 255, 255))) // Белый цвет текста
		// 	кисть = win.CreateSolidBrush(win.RGB(29, 13, 41))
		// }
		// break
		// }
		// }

		return кисть
	})
	// defer кисть.DeleteObject()

	// ВыводОшибки(" ОписаниеОшибки %+v \n", кисть.DeleteObject().Error())

}

var ОсновноеОкноПодсказок *ОкноПодсказок

func НовоеОкноПодсказок() {

	кисть := win.CreateSolidBrush(win.RGB(63, 39, 81))

	окно := ui.NewWindowMain(
		ui.WindowMainOpts().
			Title("ПотоК").
			ClientArea(win.SIZE{Cx: 300, Cy: 50}).
			WndStyles(co.WS_POPUP).
			WndExStyles(co.WS_EX_TOOLWINDOW | co.WS_EX_NOACTIVATE | co.WS_EX_TOPMOST | co.WS_EX_LAYERED).
			HBrushBkgnd(кисть),
	)
	окно.On().WmShowWindow(func(p wm.ShowWindow) {
		hwnd := окно.Hwnd()
		hwnd.SetLayeredWindowAttributes(0, 190, 0x00000002)
	})
	блокДляПодсказок := ui.NewStatic(окно,
		ui.StaticOpts().
			Text("Нажатые клавиши появятся здесь").
			Position(win.POINT{X: 10, Y: 10}).
			Size(win.SIZE{Cx: 280, Cy: 30}).
			CtrlStyles(co.SS_CENTER),
	)

	окно.On().WmCtlColorStatic(func(p wm.CtlColor) win.HBRUSH {
		hdc := p.Hdc()
		hdc.SetBkColor(win.RGB(29, 13, 41))
		hdc.SetBkMode(co.BKMODE_OPAQUE)
		SetTextColor.Call(uintptr(hdc), uintptr(win.RGB(255, 255, 255)))
		кисть := win.CreateSolidBrush(win.RGB(29, 13, 41))
		return кисть
	})

	СобытиеПеретаскивание(окно)
	ОсновноеОкноПодсказок = &ОкноПодсказок{
		окно:    окно,
		надпись: блокДляПодсказок,
	}

}

func НовоеОкно() *ПраймОкно {

	кисть := win.CreateSolidBrush(win.RGB(63, 39, 81))

	log.Printf(" %+v \n", кисть)
	окно := ui.NewWindowMain(
		ui.WindowMainOpts().
			Title("ПотоК").
			ClientArea(win.SIZE{Cx: 300, Cy: 300}).
			WndStyles(co.WS_BORDER | co.WS_SIZEBOX).
			WndExStyles(co.WS_EX_TOPMOST | co.WS_EX_LAYERED).HBrushBkgnd(кисть),
	)
	сетка := НоваяСетка(окно, 2, 1, Отступ{5, 5, 5, 5})

	основноеОкнаПрограммы := &ПраймОкно{
		окно: окно,
		надпись: ui.NewStatic(окно,
			ui.StaticOpts().
				// Text("Нажатые клавиши появятся здесь").
				Position(win.POINT{X: 10, Y: 10}).
				Size(win.SIZE{Cx: 290, Cy: 30}).
				CtrlStyles(co.SS_CENTER),
		),
		// кнопки: make([]ui.Button, len(Клавиатура)),
		статик:          make(map[ВиртуальныйКод]ui.Static, len(Клавиатура)),
		состояниеКнопок: make(map[ВиртуальныйКод]bool),
	}
	Инфо("len(Клавиатура) %+v \n", len(Клавиатура))

	Контейнер := КонтейнерЭлементов{
		строки:   1,
		столбцы:  1,
		отступ:   Отступ{5, 5, 5, 5},
		элементы: []*ui.Static{&основноеОкнаПрограммы.надпись},
	}
	// Контейнер = сетка.ДобавитьЭлементВКонтейнер(Контейнер, &основноеОкнаПрограммы.надпись)
	// сетка.ДобавитьЭлемент(&основноеОкнаПрограммы.надпись)
	сетка.ДобавитьКонтейнер(&Контейнер)

	var КонтейнерКнопок []*ui.Static
	КонтейнерДляКнопок := КонтейнерЭлементов{
		строки:        3,
		столбцы:       4,
		отступ:        Отступ{5, 5, 5, 5},
		элементы:      []*ui.Static{},
		распределение: пространствоРавномерно,
	}
	// сетка.ДобавитьЭлемент(&основноеОкнаПрограммы.надпись)

	// Создаем кнопки клавиатуры
	for _, виртуальныйКод := range Клавиатура {
		кнопка := Алвафит[виртуальныйКод]
		ру := strings.Join(кнопка.буквы["ру"], " ")
		en := strings.Join(кнопка.буквы["en"], " ")
		НадписьКнопки := fmt.Sprintf("%s\n %s\n%s", кнопка.строкаКода, ру, en)

		НовыйЭлемент := ui.NewStatic(окно,
			ui.StaticOpts().
				Text(НадписьКнопки).
				// Position(win.POINT{X: x, Y: y}).
				// Size(win.SIZE{Cx: w, Cy: h}).
				WndStyles(co.WS_CHILD|co.WS_VISIBLE|co.WS_BORDER|co.WS(co.SS_CENTER)|co.WS(co.SS_NOTIFY)),
		)

		НовыйЭлемент.On().StnClicked(func() {
			// Преобразуем координаты клиентской области в экранные
			позиция := win.POINT{X: 0, Y: 0}
			окно.Hwnd().ClientToScreenPt(&позиция)

			// Отправляем сообщение системе, что было нажатие на заголовок окна
			окно.Hwnd().SendMessage(
				co.WM_NCLBUTTONDOWN,
				win.WPARAM(co.HT_CAPTION),
				win.LPARAM(win.MAKELONG(uint16(позиция.X), uint16(позиция.Y))),
			)
		})

		основноеОкнаПрограммы.статик[кнопка.код] = НовыйЭлемент
		КонтейнерКнопок = append(КонтейнерКнопок, &НовыйЭлемент)

		// сетка.ДобавитьЭлемент(&НовыйЭлемент)
		// Обновляем элемент, чтобы изменения вступили в силу
		// hwndСтатик.InvalidateRect(nil, true)
		//hwndСтатик.ReleaseDC(hdc)
	}
	КонтейнерДляКнопок.элементы = КонтейнерКнопок
	сетка.ДобавитьКонтейнер(&КонтейнерДляКнопок)
	основноеОкнаПрограммы.сетка = *сетка

	// Добавляем обработчик для перетаскивания окна
	// окно.On().WmLButtonDown(func() {
	// 	ReleaseCapture.Call()
	// 	окно.Hwnd().SendMessage(co.WM_NCLBUTTONDOWN, 2, 0) // 2 = HTCAPTION
	// })
	// дочернееОкно := ui.NewWindowMain(
	// 	ui.WindowMainOpts().
	// 		Title("Дочернее окно").
	// 		ClientArea(win.SIZE{Cx: 200, Cy: 100}).
	// 		WndStyles(co.WS_CHILD | co.WS_VISIBLE),
	// )
	// дочернееОкно.Hwnd().ShowWindow(co.SW_SHOW)
	// дочернееОкно.Hwnd().UpdateWindow()
	СобытиеПеретаскивание(основноеОкнаПрограммы.окно)
	основноеОкнаПрограммы.ПриОтображении()
	основноеОкнаПрограммы.ИзменениеЦветаКнопок()

	return основноеОкнаПрограммы
}

type Сетка struct {
	окно ui.WindowMain
	// элементы        [][]*ui.Static
	контейнеры      []*КонтейнерЭлементов
	строки, столбцы int32
	отступ          Отступ
	распределение   Распределение
}
type КонтейнерЭлементов struct {
	строки, столбцы int32
	отступ          Отступ
	элементы        []*ui.Static
	распределение   Распределение
}

type Распределение int

const (
	безИзменений Распределение = iota
	центр
	лево
	право
	растянуть
	пространствоМежду
	пространствоРавномерно
	пространствоВокруг
)

/*
"пространствоМежду" Элементы равномерно распределяются по главной оси, при этом первый элемент находится в начале, а последний — в конце.

"пространствоРавномерно" Элементы равномерно распределяются по главной оси, при этом свободное пространство между элементами и между элементами и краями контейнера одинаково.

	"пространствоВокруг" Элементы равномерно распределяются по главной оси, при этом свободное пространство вокруг каждого элемента (до соседних элементов и краев контейнера) одинаково. Это означает, что пространство между элементами в два раза больше, чем пространство между элементами и краями контейнера.
*/
type Отступ struct {
	верхний, нижний, левый, правый int32
}

func НоваяСетка(окно ui.WindowMain, строки, столбцы int32, отступы Отступ) *Сетка {

	return &Сетка{окно: окно, строки: строки, столбцы: столбцы, отступ: отступы}
}

func (сетка *Сетка) ДобавитьКонтейнер(контейнерЭлементов *КонтейнерЭлементов) {
	// Инфо("ДобавитьКонтейнер %+v  %+v \n", сетка.контейнеры, контейнерЭлементов)

	сетка.контейнеры = append(сетка.контейнеры, контейнерЭлементов)
}

func (КонтейнерЭлементов *КонтейнерЭлементов) ДобавитьЭлементВКонтейнер(элемент *ui.Static) *КонтейнерЭлементов {
	КонтейнерЭлементов.элементы = append(КонтейнерЭлементов.элементы, элемент)
	return КонтейнерЭлементов
}
func (сетка *Сетка) Разместить() {
	// Инфо(" Разместить %+v \n", сетка)

	размерыОкна := ОсновноеОкноПрограммы.окно.Hwnd().GetClientRect()
	ширинаОкна := размерыОкна.Right - размерыОкна.Left
	высотаОкна := размерыОкна.Bottom - размерыОкна.Top

	текущаяПоложениеСВерху := сетка.отступ.верхний
	// Инфо("размерыОкна %v ширинаОкна %v высотаОкна %v текущаяПоложениеСВерху %v \n", размерыОкна, ширинаОкна, высотаОкна, текущаяПоложениеСВерху)
	// Инфо(" len(сетка.контейнеры) %+v \n", len(сетка.контейнеры))

	for _, контейнер := range сетка.контейнеры {
		// Инфо("номерКОнтейнера  %+v контейнер %+v \n", номерКОнтейнера, контейнер)

		ширинаЭлемента := (ширинаОкна - контейнер.отступ.левый - контейнер.отступ.правый) / контейнер.столбцы
		высотаЭлемента := (высотаОкна - контейнер.отступ.верхний - контейнер.отступ.нижний) / контейнер.строки

		// Инфо(" 1 высотаЭлемента  %+v ширинаЭлемента %+v \n", высотаЭлемента, ширинаЭлемента)

		for i, элемент := range контейнер.элементы {
			эл := *элемент
			строка := int32(i) / контейнер.столбцы
			столбец := int32(i) % контейнер.столбцы

			ВысотаСвободноОбласти := высотаОкна - текущаяПоложениеСВерху - контейнер.отступ.нижний

			x := контейнер.отступ.левый + столбец*(ширинаЭлемента+контейнер.отступ.правый)
			y := текущаяПоложениеСВерху + контейнер.отступ.верхний + строка*(высотаЭлемента+контейнер.отступ.нижний)

			// Проверяем, заданы ли размеры у элемента
			размерыЭлемента := эл.Hwnd().GetClientRect()
			ширинаЭлемента = размерыЭлемента.Right - размерыЭлемента.Left
			высотаЭлемента = размерыЭлемента.Bottom - размерыЭлемента.Top

			// Инфо("2 высотаЭлемента  %+v ширинаЭлемента %+v \n", высотаЭлемента, ширинаЭлемента)

			if ширинаЭлемента == 0 || высотаЭлемента == 0 {
				ширинаЭлемента = (ширинаОкна - контейнер.отступ.левый - контейнер.отступ.правый) / контейнер.столбцы
				высотаЭлемента = (ВысотаСвободноОбласти - контейнер.отступ.верхний - контейнер.отступ.нижний) / контейнер.строки
			}
			// Инфо("высотаЭлемента  %+v ширинаЭлемента %+v \n", высотаЭлемента, ширинаЭлемента)
			// Применяем распределение, если оно задано
			if контейнер.распределение > 0 {
				switch контейнер.распределение {
				case центр:
					x += (ширинаОкна - ширинаЭлемента) / 2
					y += (ВысотаСвободноОбласти - высотаЭлемента) / 2
				case лево:
					x = контейнер.отступ.левый
				case право:
					x = ширинаОкна - ширинаЭлемента - контейнер.отступ.правый
				case растянуть:
					ширинаЭлемента = ширинаОкна - контейнер.отступ.левый - контейнер.отступ.правый
					высотаЭлемента = ВысотаСвободноОбласти - контейнер.отступ.верхний - контейнер.отступ.нижний
				case пространствоМежду:
					// Равномерное распределение с учетом отступов
					ширинаЭлемента = (ширинаОкна - контейнер.отступ.левый - контейнер.отступ.правый - (контейнер.столбцы-1)*контейнер.отступ.правый) / контейнер.столбцы
					высотаЭлемента = (ВысотаСвободноОбласти - контейнер.отступ.верхний - контейнер.отступ.нижний - (контейнер.строки-1)*контейнер.отступ.нижний) / контейнер.строки
				case пространствоРавномерно:
					// Равномерное распределение с учетом отступов
					ширинаЭлемента = (ширинаОкна - контейнер.отступ.левый - контейнер.отступ.правый - (контейнер.столбцы-1)*контейнер.отступ.правый) / контейнер.столбцы
					высотаЭлемента = (ВысотаСвободноОбласти - контейнер.отступ.верхний - контейнер.отступ.нижний - (контейнер.строки-1)*контейнер.отступ.нижний) / контейнер.строки
				case пространствоВокруг:
					// Равномерное распределение с учетом отступов
					ширинаЭлемента = (ширинаОкна - контейнер.отступ.левый - контейнер.отступ.правый - (контейнер.столбцы-1)*контейнер.отступ.правый) / контейнер.столбцы
					высотаЭлемента = (ВысотаСвободноОбласти - контейнер.отступ.верхний - контейнер.отступ.нижний - (контейнер.строки-1)*контейнер.отступ.нижний) / контейнер.строки
				}
			}

			// Отладочная информация
			// Инфо("Элемент %d: x=%d, y=%d, ширинаЭлемента=%d, высотаЭлемента=%d текущаяПоложениеСВерхуОтВерхаОкна=%d  \n", i, x, y, ширинаЭлемента, высотаЭлемента, текущаяПоложениеСВерху)

			эл.Hwnd().MoveWindow(x, y, ширинаЭлемента, высотаЭлемента, true)
		}

		текущаяПоложениеСВерху += контейнер.отступ.верхний + контейнер.отступ.нижний + высотаЭлемента*контейнер.строки

	}
}

type RECT struct {
	Left, Top, Right, Bottom int32
}
