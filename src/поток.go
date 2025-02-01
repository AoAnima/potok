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

// func вводБуквы(буква string) {
// 	// Создаем массив для хранения Unicode символов
// 	руны := []rune(буква)

// 	for _, руна := range руны {
// 		var input INPUT
// 		input.Type = INPUT_KEYBOARD
// 		// kb := (*KEYBDINPUT)(unsafe.Pointer(&input.Data[0]))
// 		kb.WVk = 0
// 		kb.WScan = uint16(руна) // Используем Unicode символ
// 		kb.DwFlags = KEYEVENTF_UNICODE
// 		kb.Time = 0
// 		kb.DwExtraInfo = 0

// 		// Нажатие клавиши
// 		SendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))

// 		// Отпускание клавиши
// 		kb.DwFlags |= KEYEVENTF_KEYUP
// 		SendInput.Call(1, uintptr(unsafe.Pointer(&input)), unsafe.Sizeof(input))
// 	}
// }

type СтруктураКлавиатурногоХука struct {
	ВиртуальныйКод           ВиртуальныйКод
	СканКод                  СканКод
	Флаги                    uint32
	Время                    uint32
	ДополнительнаяИнформация uintptr
}
type ВиртуальныйКод uint32
type СканКод uint32

type Кнопка struct {
	x, y, w, h float64
	код        ВиртуальныйКод
	строкаКода string
	буквы      map[string][]string
}

type ПраймОкно struct {
	окно            ui.WindowMain
	надпись         ui.Static
	кнопки          []ui.Button
	статик          map[ВиртуальныйКод]ui.Static
	состояниеКнопок map[ВиртуальныйКод]bool // Добавляем поле для хранения состояния кнопок
	сетка           Сетка
}

type ОкноПодсказок struct {
	окно            ui.WindowMain
	надпись         ui.Static
	статик          map[string]ui.Static
	состояниеКнопок map[ВиртуальныйКод]bool // Добавляем поле для хранения состояния кнопок
	сетка           Сетка
}

var Клавиатура = []Кнопка{

	{0.1, 0.1, 0.18, 0.15, 0x51, "0x51", map[string][]string{"en": []string{"E", "T"}, "ру": []string{"И", "Б", "Ы"}}},
	{0.32, 0.1, 0.18, 0.15, 0x57, "0x57", map[string][]string{"en": []string{"A", "O"}, "ру": []string{"В", "Ь", "Ъ"}}},
	{0.54, 0.1, 0.18, 0.15, 0x45, "0x45", map[string][]string{"en": []string{"I", "N"}, "ру": []string{"Д", "Е", "Ё"}}},
	{0.76, 0.1, 0.18, 0.15, 0x52, "0x52", map[string][]string{"en": []string{"S", "H"}, "ру": []string{"Ж", "З", "Н"}}},

	// Второй ряд (4 кнопки)
	{0.1, 0.3, 0.18, 0.15, 0x41, "0x41", map[string][]string{"en": []string{"R", "D"}, "ру": []string{"A", "Й"}}},
	{0.32, 0.3, 0.18, 0.15, 0x53, "0x53", map[string][]string{"en": []string{"L", "C"}, "ру": []string{"К", "Л"}}},
	{0.54, 0.3, 0.18, 0.15, 0x44, "0x44", map[string][]string{"en": []string{"U", "M"}, "ру": []string{"М", "П"}}},
	{0.76, 0.3, 0.18, 0.15, 0x46, "0x46", map[string][]string{"en": []string{"W", "F"}, "ру": []string{"О", "Р"}}},

	// Третий ряд (4 кнопки)
	{0.1, 0.5, 0.18, 0.15, 0x5A, "0x5A", map[string][]string{"en": []string{"G", "Y"}, "ру": []string{"Ф", "Х", "Э", "Ю"}}},
	{0.32, 0.5, 0.18, 0.15, 0x58, "0x58", map[string][]string{"en": []string{"P", "B"}, "ру": []string{"Ц", "Ч", "Ш", "Щ"}}},
	{0.54, 0.5, 0.18, 0.15, 0x43, "0x43", map[string][]string{"en": []string{"V", "K"}, "ру": []string{"Р", "С"}}},
	{0.76, 0.5, 0.18, 0.15, 0x56, "0x56", map[string][]string{"en": []string{"J", "X", "Q", "Z"}, "ру": []string{"Т", "У"}}},
}

// Канал для обновления UI
var каналОбновленияОкна = make(chan СтруктураКлавиатурногоХука, 100)
var ОсновноеОкноПрограммы *ПраймОкно

func main() {
	runtime.LockOSThread()

	ОсновноеОкноПрограммы = НовоеОкно()
	ТестВывода()
	// hwnd := Окно.окно.Hwnd()
	// fmt.Printf("hwnd: %v\n", hwnd)
	//hwnd.SetLayeredWindowAttributes(0, 255, 0x00000002)
	// SetLayeredWindowAttributes.Call(
	// 	uintptr(hwnd),
	// 	0,
	// 	255,        // Уровень прозрачности (0-255)
	// 	0x00000002, // LWA_ALPHA
	// )

	// Горутина для обновления UI
	go func() {

		for структураКлавиатурногоХука := range каналОбновленияОкна {
			func(структураКлавиатурногоХука СтруктураКлавиатурногоХука) {
				runtime.LockOSThread()
				// hwnd := Окно.окно.Hwnd()
				// fmt.Printf("hwnd: %v\n", hwnd)
				// hwnd.SetLayeredWindowAttributes(0, 190, 0x00000002)
				ВиртуальныйКод := структураКлавиатурногоХука.ВиртуальныйКод
				var состояниеКлавишы [256]byte
				ПолучитьСостояниеКлавиатуры.Call(uintptr(unsafe.Pointer(&состояниеКлавишы[0])))

				var буква [2]uint16
				if колВо, _, _ := кодВСимвол.Call(
					uintptr(ВиртуальныйКод),
					uintptr(структураКлавиатурногоХука.СканКод),
					uintptr(unsafe.Pointer(&состояниеКлавишы[0])),
					uintptr(unsafe.Pointer(&буква[0])),
					2,
					0,
					0); колВо > 0 {
					// hwnd := ОсновноеОкноПрограммы.окно.Hwnd()
					// Инфо("hwnd: %v\n", hwnd)
					буква := string(utf16.Decode(буква[:колВо]))
					ОсновноеОкноПодсказок.надпись.SetText(fmt.Sprintf("ВиртуальныйКод %v буква %s", ВиртуальныйКод, буква))
					// ПозицияКаретки := win.GetCaretPos()
					// Инфо("ПозицияКаретки: %\n", ПозицияКаретки)

					// ПозицияКурсора := win.GetCursorPos()
					// Инфо("ПозицияКурсора: %\n", ПозицияКурсора)

					// Инфо("Код клавиши: 0x%X, Символ: %s\n", ВиртуальныйКод, char)
					// Инфо("структураКлавиатурногоХука.ВиртуальныйКод: %v, Символ: %s\n", структураКлавиатурногоХука.ВиртуальныйКод, char)
					// каналОбновленияОкна <- fmt.Sprintf("Код клавиши: 0x%X, Символ: %s", vkCode, char)
					// Создаем новый статический элемент с введенным символом
					// дочернееОкно := ui.NewWindowMain(
					// 	ui.WindowMainOpts().
					// 		Title("ПотоК").
					// 		ClientArea(win.SIZE{Cx: 300, Cy: 300}).
					// 		WndStyles(co.WS_POPUP | co.WS_BORDER | co.WS_SIZEBOX | co.WS_VISIBLE),
					// 	// WndExStyles(co.WS_EX_TOPMOST | co.WS_EX_LAYERED).HBrushBkgnd(кисть),
					// )
					// показано := дочернееОкно.Hwnd().ShowWindow(co.SW_SHOW)
					// Инфо(" %+v \n", показано)
					// var caret win.RECT
					// caret = win.GetCaretPos()
					// Инфо("caret %+v \n", caret)
					// ret, r1, r2 := GetLastError.Call()
					// Инфо("GetLastError %+v  %+v  %+v \n", ret, r1, r2)

					// ОсновноеОкноПодсказок.надпись.SetText(fmt.Sprintf("caret: %v, ", caret))

					// показано = дочернееОкно.Hwnd().UpdateWindow()
					// Инфо(" %+v \n", показано)
					// ОсновноеОкноПодсказок.надпись.SetText(fmt.Sprintf("Код клавиши: 0x%X, Символ: %s", ВиртуальныйКод, char))
					// ОсновноеОкноПрограммы.надпись.SetText(fmt.Sprintf("Код клавиши: 0x%X, Символ: %s", ВиртуальныйКод, char))
					// for _, кнопка := range Клавиатура {
					// 	if кнопка.код == ВиртуальныйКод {
					// 		// Получаем две буквы для ввода
					// 		буквы := кнопка.буквы["ру"]
					// 		if len(буквы) >= 2 {
					// 			вводБуквы(буквы[0])
					// 			вводБуквы(буквы[1])
					// 		}
					// 		break
					// 	}
					// }

				} else {
					Инфо("Код клавиши: 0x%X\n", ВиртуальныйКод)
					ОсновноеОкноПрограммы.надпись.SetText(fmt.Sprintf("Код клавиши: 0x%X", ВиртуальныйКод))
					// каналОбновленияОкна <- fmt.Sprintf("Код клавиши: 0x%X", vkCode)
				}

				// Окно.надпись.SetText(структураКлавиатурногоХука.ВиртуальныйКод)
				// делаем цвет кнопки светлее а через 1 секунду востанавливаем
				if статикКнопка, ок := ОсновноеОкноПрограммы.статик[структураКлавиатурногоХука.ВиртуальныйКод]; ок {
					ОсновноеОкноПрограммы.состояниеКнопок[структураКлавиатурногоХука.ВиртуальныйКод] = true
					hwndСтатик := статикКнопка.Hwnd()
					hwndСтатик.InvalidateRect(nil, true)
					// ОсновноеОкноПрограммы.сетка.Разместить()
					// Через 1 секунду возвращаем исходные цвета
					time.AfterFunc(200*time.Millisecond, func() {
						ОсновноеОкноПрограммы.состояниеКнопок[структураКлавиатурногоХука.ВиртуальныйКод] = false
						hwndСтатик.InvalidateRect(nil, true)
					})

				}
				runtime.UnlockOSThread()
			}(структураКлавиатурногоХука)
		}
	}()
	// win.SetWindowsHookEx(co.WH_MOUSE, func(code int32, wp win.WPARAM, lp win.LPARAM) uintptr {

	// 	hwndForeground := win.GetForegroundWindow()
	// 	Инфо(" %+v \n", hwndForeground)
	// 	ret, _, _ := СледующийХук.Call(0, uintptr(code), uintptr(wp), uintptr(lp))
	// 	return ret
	// }, 0, 0)

	win.SetWindowsHookEx(co.WH_KEYBOARD_LL, func(code int32, wp win.WPARAM, lp win.LPARAM) uintptr {
		if code >= 0 && wp == win.WPARAM(co.WM_KEYDOWN) {
			структураКлавиатуры := (*СтруктураКлавиатурногоХука)(unsafe.Pointer(lp))

			for _, кнопка := range Клавиатура {
				if кнопка.код == структураКлавиатуры.ВиртуальныйКод {
					// Получаем буквы для текущего языка
					if буквы, есть := кнопка.буквы["ру"]; есть {
						// Эмулируем ввод каждой буквы в активное окно 
						for _, буква := range буквы {
							Инфо("буква %v", буква)
							вводБуквыВАктивноеОкно(буква)
						}
					}
					break
				}
			}
		}
		ret, _, _ := СледующийХук.Call(0, uintptr(code), uintptr(wp), uintptr(lp))
		return ret
	}, 0, 0)

	go func() {
		НовоеОкноПодсказок()

		дескриптор := ОсновноеОкноПодсказок.окно.RunAsMain()
		Инфо("дескриптор %+v \n", дескриптор)

	}()
	// курсорПозиция := win.GetCursorPos()

	// // Создаем новое окно и размещаем его под курсором
	// // Создаем новое окно
	// новоеОкно := ui.NewWindowMain(
	// 	ui.WindowMainOpts().
	// 		Title("Новое окно").
	// 		ClientArea(win.SIZE{Cx: 200, Cy: 150}),
	// )
	// новоеОкно.Hwnd().ShowWindow(co.SW_SHOW)
	// новоеОкно.Hwnd().UpdateWindow()
	// Устанавливаем позицию нового окна под курсором
	// новоеОкно.Hwnd().MoveWindow(курсорПозиция.X, курсорПозиция.Y, 200, 150, true)
	// дочернееОкно := ui.NewWindowMain(
	// 	ui.WindowMainOpts().
	// 		Title("Дочернее окно").
	// 		ClientArea(win.SIZE{Cx: 200, Cy: 100}).
	// 		WndStyles(co.WS_CHILD | co.WS_VISIBLE),
	// )

	ОсновноеОкноПрограммы.окно.RunAsMain()
	log.Println(" Окно ")

	close(каналОбновленияОкна)
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

func ПрисоединитьПотоки(текущийПоток, целевойПоток uint32, присоединить bool) bool {
	присоединитьЗначение := 0
	if присоединить {
		присоединитьЗначение = 1
	}

	ret, ret2, err := AttachThreadInput.Call(
		uintptr(текущийПоток),
		uintptr(целевойПоток),
		uintptr(присоединитьЗначение),
	)
	Инфо("ПрисоединитьПотоки %+v  %+v  %+v \n", ret, ret2, err)

	return ret != 0
}

func ПолучитьАктивноеОкноИКаретку() (win.HWND, win.RECT) {
	// Получаем хендл активного окна
	активноеОкно := win.GetForegroundWindow()

	// Получаем ID потока активного окна
	_, активныйПоток := активноеОкно.GetWindowThreadProcessId()

	// Получаем ID текущего потока
	текущийПоток := win.GetCurrentThreadId()

	// Присоединяемся к потоку активного окна для получения информации о каретке
	ПрисоединитьПотоки(текущийПоток, активныйПоток, true)
	defer ПрисоединитьПотоки(текущийПоток, активныйПоток, false)

	// Получаем позицию каретки
	// var позицияКаретки win.RECT
	позицияКаретки := win.GetCaretPos()
	Инфо("1 позицияКаретки %v активноеОкно %v", позицияКаретки, активноеОкно)
	// Преобразуем координаты каретки в экранные координаты
	активноеОкно.ClientToScreenRc(&позицияКаретки)
	Инфо("2 позицияКаретки %v активноеОкно %v", позицияКаретки, активноеОкно)
	return активноеОкно, позицияКаретки
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

// func ShowDesktop() {
// 	// Создаём массив из 4 структур INPUT
// 	inputs := make([]INPUT, 4)

// 	// Нажатие клавиши "Windows"
// 	inputs[0].Type = INPUT_KEYBOARD
// 	kbPressWin := (*KEYBDINPUT)(unsafe.Pointer(&inputs[0].Data[0]))
// 	kbPressWin.WVk = VK_LWIN

// 	// Нажатие клавиши "D"
// 	inputs[1].Type = INPUT_KEYBOARD
// 	kbPressD := (*KEYBDINPUT)(unsafe.Pointer(&inputs[1].Data[0]))
// 	kbPressD.WVk = 'D'

// 	// Отпускание клавиши "D"
// 	inputs[2].Type = INPUT_KEYBOARD
// 	kbReleaseD := (*KEYBDINPUT)(unsafe.Pointer(&inputs[2].Data[0]))
// 	kbReleaseD.WVk = 'D'
// 	kbReleaseD.DwFlags = KEYEVENTF_KEYUP

// 	// Отпускание клавиши "Windows"
// 	inputs[3].Type = INPUT_KEYBOARD
// 	kbReleaseWin := (*KEYBDINPUT)(unsafe.Pointer(&inputs[3].Data[0]))
// 	kbReleaseWin.WVk = VK_LWIN
// 	kbReleaseWin.DwFlags = KEYEVENTF_KEYUP

// 	// Вызываем SendInput для отправки всех событий
// 	r1, r2, err := SendInput.Call(
// 		uintptr(len(inputs)),             // Количество структур в массиве
// 		uintptr(unsafe.Pointer(&inputs)), // Указатель на массив структур
// 		uintptr(unsafe.Sizeof(inputs)),   // Размер одной структуры INPUT
// 	)
// 	Инфо(" ShowDesktop%+v %+v %+v \n", r1, r2, err)

// 	if r1 != uintptr(len(inputs)) {
// 		ВыводОшибки("SendInput failed: %м", err)
// 	}
// }

// func  ShowDesktop() {
//     // OutputString(L"Sending 'Win-D'\r\n");
//     INPUT inputs[4] = {};
//     ZeroMemory(inputs, sizeof(inputs));

//     inputs[0].type = INPUT_KEYBOARD;
//     inputs[0].ki.wVk = VK_LWIN;

//     inputs[1].type = INPUT_KEYBOARD;
//     inputs[1].ki.wVk = 'D';

//     inputs[2].type = INPUT_KEYBOARD;
//     inputs[2].ki.wVk = 'D';
//     inputs[2].ki.dwFlags = KEYEVENTF_KEYUP;

//     inputs[3].type = INPUT_KEYBOARD;
//     inputs[3].ki.wVk = VK_LWIN;
//     inputs[3].ki.dwFlags = KEYEVENTF_KEYUP;

//     UINT uSent = SendInput(ARRAYSIZE(inputs), inputs, sizeof(INPUT));
//     if (uSent != ARRAYSIZE(inputs))
//     {
//         OutputString(L"SendInput failed: 0x%x\n", HRESULT_FROM_WIN32(GetLastError()));
//     }
// }

func ТестВывода() {

	inputs := make([]INPUT, 4)

	inputs[0].Type = INPUT_KEYBOARD
	inputs[0].Ki = KEYBDINPUT{
		WVk: VK_LWIN,
	}

	inputs[1].Type = INPUT_KEYBOARD
	inputs[1].Ki = KEYBDINPUT{
		WVk: 'D',
		// WScan:       uint16(руна),
		// DwFlags:     KEYEVENTF_UNICODE,
		// Time:        0,
		// DwExtraInfo: 0,
	}
	inputs[2].Type = INPUT_KEYBOARD
	inputs[2].Ki = KEYBDINPUT{
		WVk: 'D',
		// WScan:       uint16(руна),
		DwFlags: KEYEVENTF_KEYUP,
		// Time:        0,
		// DwExtraInfo: 0,
	}
	inputs[3].Type = INPUT_KEYBOARD
	inputs[3].Ki = KEYBDINPUT{
		WVk: VK_LWIN,
		// WScan:       uint16(руна),
		DwFlags: KEYEVENTF_KEYUP,
		// Time:        0,
		// DwExtraInfo: 0,
	}

	рез, рез2, ош := SendInput.Call(
		uintptr(len(inputs)),
		uintptr(unsafe.Pointer(&inputs)),
		unsafe.Sizeof(inputs[0]),
	)
	Инфо(" %+v %+v %+v \n", len(inputs), inputs, int32(unsafe.Sizeof(inputs[0])))

	Инфо(" %+v  %+v  %+v \n", рез, рез2, ош)

}

func вводБуквыВАктивноеОкно(буква string) {

	// активноеОкно, _ := ПолучитьАктивноеОкноИКаретку()
	// Инфо("вводБуквыВАктивноеОкно %v", активноеОкно)

	// if !активноеОкно.IsWindow() || !активноеОкно.IsWindowVisible() {
	// 	return
	// }

	букваДляВывода := utf16.Encode([]rune(буква))
	Инфо("букваДляВывода %+v \n", букваДляВывода)

	inputs := make([]INPUT, 0, len(букваДляВывода))

	for _, руна := range букваДляВывода {
		Инфо("руна %v", руна)

		// Заполняем структуру правильно
		inputs = append(inputs, INPUT{
			Type: INPUT_KEYBOARD,
			Ki: KEYBDINPUT{
				//WVk:         uint16(руна),
				WScan:   uint16(руна),
				DwFlags: KEYEVENTF_UNICODE,

				// DwExtraInfo: 0,
			}})

		// Фокусируем окно перед вводом
		// установленоАктивноеОкно := активноеОкно.SetForegroundWindow()
		// Инфо("установленоАктивноеОкно %v", установленоАктивноеОкно)

		// Нажатие клавиши ds
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
}
func GetCaretPosSys() RECT {
	var rc RECT
	ret, r1, err := syscall.SyscallN(GetCaretPos.Addr(),
		uintptr(unsafe.Pointer(&rc)))

	Инфо(" ret %+v, r1 %+v, err %+v %+v \n", ret, r1, err, rc)

	// if ret == 0 {
	// 	panic(errco.ERROR(err))
	// }
	return rc
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
		hwnd := окно.окно.Hwnd()
		fmt.Printf("hwnd: %v\n", hwnd)
		hwnd.SetLayeredWindowAttributes(0, 190, 0x00000002)
		ОсновноеОкноПрограммы.сетка.Разместить()

	})

}

func (окно ПраймОкно) ИзменениеЦветаКнопок() {

	окно.окно.On().WmCtlColorStatic(func(p wm.CtlColor) win.HBRUSH {
		hdc := p.Hdc()

		// Устанавливаем цвет фона на фиолетовый
		hdc.SetBkColor(win.RGB(29, 13, 41))
		// Устанавливаем режим фона на OPAQUE
		hdc.SetBkMode(co.BKMODE_OPAQUE)

		SetTextColor.Call(uintptr(hdc), uintptr(win.RGB(255, 255, 255)))
		// Возвращаем дескриптор кисти, если необходимо
		кисть := win.CreateSolidBrush(win.RGB(29, 13, 41))
		hwnd := p.HwndControl()
		fmt.Printf("HwndControl: %v\n", hwnd)
		// Начало добавления

		for код, статикКнопка := range окно.статик {
			if статикКнопка.Hwnd() == hwnd {
				if окно.состояниеКнопок[код] {
					hdc.SetBkColor(win.RGB(255, 0, 123))
					SetTextColor.Call(uintptr(hdc), uintptr(win.RGB(139, 234, 0)))
					кисть = win.CreateSolidBrush(win.RGB(255, 0, 123))
				} else {
					hdc.SetBkColor(win.RGB(29, 13, 41))
					// Фиолетовый цвет
					SetTextColor.Call(uintptr(hdc), uintptr(win.RGB(255, 255, 255))) // Белый цвет текста
					кисть = win.CreateSolidBrush(win.RGB(29, 13, 41))
				}
				break
			}
		}
		return кисть
	})
}

var ОсновноеОкноПодсказок *ОкноПодсказок

func НовоеОкноПодсказок() {

	кисть := win.CreateSolidBrush(win.RGB(63, 39, 81))
	окно := ui.NewWindowMain(
		ui.WindowMainOpts().
			Title("ПотоК").
			ClientArea(win.SIZE{Cx: 300, Cy: 50}).
			WndStyles(co.WS_POPUP).
			WndExStyles(co.WS_EX_TOPMOST | co.WS_EX_LAYERED).
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
	for _, кнопка := range Клавиатура {

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
