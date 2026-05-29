package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	. "aoanima.ru/Logger"
	"aoanima.ru/QErrors"

	"github.com/rodrigocfd/windigo/ui"
	"github.com/rodrigocfd/windigo/ui/wm"
	"github.com/rodrigocfd/windigo/win"
	"github.com/rodrigocfd/windigo/win/co"

	"net/http"
	_ "net/http/pprof"
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
	ReleaseCapture           = user32.NewProc("ReleaseCapture")
	SetTextColor             = gdi32.NewProc("SetTextColor")
	GetLastError             = Kernel32.NewProc("GetLastError")
	SendInput                = user32.NewProc("SendInput")
	GetGUIThreadInfo         = user32.NewProc("GetGUIThreadInfo")
	GetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	GetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	CallWindowProc           = user32.NewProc("CallWindowProcW")
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
	hwnd   win.HWND
	дети   []win.HWND
	тексты []string
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

	WM_UPDATE_PREDICTIONS = 0x8000 // WM_APP
)

var оригинальнаяПроцедураГлавногоОкна uintptr

func новаяПроцедураГлавногоОкна(hwnd win.HWND, msg uint32, wParam win.WPARAM, lParam win.LPARAM) uintptr {
	if msg == WM_UPDATE_PREDICTIONS {
		if ОсновноеОкноПодсказок != nil {
			ОсновноеОкноПодсказок.ОбновитьПредсказания(ТекущиеПредсказания, ВыбранноеПредсказаниеИндекс)
		}
		return 0
	}
	ret, _, _ := CallWindowProc.Call(оригинальнаяПроцедураГлавногоОкна, uintptr(hwnd), uintptr(msg), uintptr(wParam), uintptr(lParam))
	return ret
}

var Алвафит = map[ВиртуальныйКод]Кнопка{

	0x51: {0x51, "0x51", map[string][]string{"en": []string{"E", "T"}, "ру": []string{"И", "Б", "Ы"}}},
	0x57: {0x57, "0x57", map[string][]string{"en": []string{"A", "O"}, "ру": []string{"В", "Ь", "Ъ"}}},
	0x45: {0x45, "0x45", map[string][]string{"en": []string{"I", "N"}, "ру": []string{"Д", "Е", "Ё"}}},
	0x52: {0x52, "0x52", map[string][]string{"en": []string{"S", "H"}, "ру": []string{"Ж", "З", "Н"}}},
	0x43: {0x43, "0x43", map[string][]string{"en": []string{".", "["}, "ру": []string{".", "["}}},

	// Второй ряд (4 кнопки)
	0x41: {0x41, "0x41", map[string][]string{"en": []string{"R", "D"}, "ру": []string{"A", "Й"}}},
	0x53: {0x53, "0x53", map[string][]string{"en": []string{"L", "C"}, "ру": []string{"К", "Л"}}},
	0x44: {0x44, "0x44", map[string][]string{"en": []string{"U", "M"}, "ру": []string{"М", "П"}}},
	0x46: {0x46, "0x46", map[string][]string{"en": []string{"W", "F"}, "ру": []string{"О", "Р"}}},
	0x56: {0x56, "0x56", map[string][]string{"en": []string{"{", "\""}, "ру": []string{"{", "\""}}},

	// Третий ряд (4 кнопки)
	0x5A: {0x5A, "0x5A", map[string][]string{"en": []string{"G", "Y"}, "ру": []string{"Ф", "Х", "Э", "Ю"}}},
	0x58: {0x58, "0x58", map[string][]string{"en": []string{"P", "B"}, "ру": []string{"Ц", "Ч", "Ш", "Щ"}}},
	0x54: {0x54, "0x54", map[string][]string{"en": []string{"V", "K"}, "ру": []string{"Р", "С"}}},
	0x47: {0x47, "0x47", map[string][]string{"en": []string{"J", "X", "Q", "Z"}, "ру": []string{"Т", "У"}}},
}

var Клавиатура = []ВиртуальныйКод{
	0x51,
	0x57,
	0x45,
	0x52,
	0x43,

	0x41,
	0x53,
	0x44,
	0x46,
	0x56,

	0x5A,
	0x58,
	0x54,
	0x47,
}

// Канал для обновления UI
var каналОбновленияОкна = make(chan ДанныеКлавиатурногоСобытия, 100)
var каналПредсказания = make(chan []ВиртуальныйКод, 100) // AiCopilot-Code начало: Изменен тип канала для передачи всего буфера

var ОсновноеОкноПрограммы *ПраймОкно
var Словари *СловариСлов               // AiCopilot-Code начало: Глобальная переменная для словарей
var КэшЧастотности *КэшЧастотностиСлов // AiCopilot-Code начало: Глобальная переменная для кэша частотности
var ТекущиеПредсказания []string       // AiCopilot-Code начало: Хранение текущих предсказаний
var ВыбранноеПредсказаниеИндекс int    // AiCopilot-Code начало: Индекс выбранного предсказания
var путьКДанным = "data"
var HWNDГлавногоОкна win.HWND // HWND главного окна (устанавливается после создания)

func main() {
	exe, err := os.Executable()
	if err == nil {
		путьКДанным = filepath.Join(filepath.Dir(exe), "..", "data")
	}
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	runtime.LockOSThread()
	ХукКлавиатуры()
	// AiCopilot-Code начало: Инициализация словарей и кэша частотности
	Словари = НовыеСловариСлов()
	статусЗагрузкиСловарей := Словари.ЗагрузитьСловари(filepath.Join(путьКДанным, "dictionaries", "russian_words.txt"), filepath.Join(путьКДанным, "dictionaries", "english_words.txt"))
	if статусЗагрузкиСловарей.Код != QErrors.Ок {
		ВыводОшибки("Ошибка загрузки словарей: %s", статусЗагрузкиСловарей.Текст)
		//return
	}

	КэшЧастотности = НовыиКэшЧастотностиСлов()
	статусЗагрузкиКэша := КэшЧастотности.ЗагрузитьКэш(filepath.Join(путьКДанным, "frequency_cache.json"))
	if статусЗагрузкиКэша.Код != QErrors.Ок {
		ВыводОшибки("Ошибка загрузки кэша частотности: %s", статусЗагрузкиКэша.Текст)
		// Продолжаем работу, так как отсутствие кэша не критично
	}
	// AiCopilot-Code конец

	ОсновноеОкноПрограммы = НовоеОкно()

	// AiCopilot-Code начало: Создаём окно подсказок через сырой Win32 на основном потоке
	НовоеОкноПодсказок()

	// Горутина для обновления UI
	go ПотокОбновленияЮИ()

	// AiCopilot-Code начало: Запуск горутины для предсказания слов
	go ПредсказаниеСлов()
	// AiCopilot-Code конец

	ОсновноеОкноПрограммы.окно.RunAsMain()
	log.Println(" Окно ")

	close(каналОбновленияОкна)
	// AiCopilot-Code начало: Сохранение кэша частотности при завершении
	статусСохраненияКэша := КэшЧастотности.СохранитьКэш(filepath.Join(путьКДанным, "frequency_cache.json"))
	if статусСохраненияКэша.Код != QErrors.Ок {
		ВыводОшибки("Ошибка сохранения кэша частотности: %s", статусСохраненияКэша.Текст)
	}
	// AiCopilot-Code конец
}

var зажатаяКлавиша = make(map[ВиртуальныйКод]bool)

func ХукКлавиатуры() {

	win.SetWindowsHookEx(co.WH_KEYBOARD_LL, func(код int32, типСобытия win.WPARAM, структураКлавишы win.LPARAM) uintptr {
		if код >= 0 {
			// if типСобытия == win.WPARAM(co.WM_KEYDOWN) {
			// Инфо("типСобытия %+v \n", типСобытия)

			структураКлавиатуры := (*СтруктураКлавиатурногоХука)(unsafe.Pointer(структураКлавишы))
			// Инфо("структураКлавиатуры %+v \n", структураКлавиатуры)

			// События, сгенерированные потоком (Backspace/text): пропускаем в окно, не перехватываем
			if структураКлавиатуры.ДополнительнаяИнформация == УказательНаПоток {
				Инфо(" структураКлавиатуры.ДополнительнаяИнформация %+v \n", структураКлавиатуры.ДополнительнаяИнформация)
				ret, _, _ := СледующийХук.Call(0, uintptr(код), uintptr(типСобытия), uintptr(структураКлавишы))
				return ret
			}

			каналОбновленияОкна <- ДанныеКлавиатурногоСобытия{
				*структураКлавиатуры,
				типСобытия,
			}
			// Инфо(" структураКлавиатуры %+v  типСобытия %+v \n", структураКлавиатуры, типСобытия)

			// Блокируем пробел от попадания в активное окно, если есть предсказания
			if структураКлавиатуры.ВиртуальныйКод == 0x20 && len(ТекущиеПредсказания) > 0 {
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

			// if СобытиеКлавиатуры.ТипСобытия == win.WPARAM(co.WM_KEYDOWN) {
			runtime.LockOSThread()

			// if колВо, буква := ВЮникод(&СобытиеКлавиатуры); колВо > 0 {

			// Инфо("буква [2]uint16 %+v \n", буква)

			// буква := string(utf16.Decode(буква[:колВо]))
			ОсновноеОкноПрограммы.надпись.SetText(fmt.Sprintf("Код клавиши: 0x%X", СобытиеКлавиатуры.ВиртуальныйКод))
			БуферНажатий(&СобытиеКлавиатуры)
			// ПечатьТекста(буква)

			// } else {
			// 	Инфо("Код клавиши: 0x%X\n", СобытиеКлавиатуры.ВиртуальныйКод)
			// 	ОсновноеОкноПрограммы.надпись.SetText(fmt.Sprintf("Код клавиши: 0x%X", СобытиеКлавиатуры.ВиртуальныйКод))
			// 	// каналОбновленияОкна <- fmt.Sprintf("Код клавиши: 0x%X", vkCode)
			// }

			// делаем цвет кнопки светлее а через 1 секунду востанавливаем
			МиганиеКнопки(&СобытиеКлавиатуры)

			runtime.UnlockOSThread()
			// }
		}(СобытиеКлавиатуры)
	}
}

type БуферКодов struct {
	ТекущееСлово []ВиртуальныйКод   // Последовательность кодов нажатых клаишь
	ВсеНаборы    [][]ВиртуальныйКод // набор всех Последовательностей кодов
	Слова        []string           // набор всех слов который были выведдены на экран

}

// var ЗажатаяКлавиша = make(map[ВиртуальныйКод]bool)
var ЗажатаяКлавиша = make(map[bool]ВиртуальныйКод)
var Буфер = БуферКодов{}

const (
	Пробел       = 0x20
	Enter        = 0x0D
	Shift        = 0xA0
	Ctrl         = 0xA2
	Alt          = 0xA4
	Backspace    = 0x08
	Tab          = 0x09
	Esc          = 0x1B
	Delete       = 0x2E
	Home         = 0x24
	End          = 0x23
	PageUp       = 0x21
	PageDown     = 0x22
	Left         = 0x25
	Up           = 0x26
	Right        = 0x27
	С            = 0x43
	М            = 0x56
	СтрелкаВверх = 0x26 // AiCopilot-Code начало: Добавлены коды для стрелок
	СтрелкаВниз  = 0x28
)

/*
БуферНажатий функция созраняем все коды нажатых клавиь в массив или буфер, если нажат пробел или шифт то происходи  вывод первого слова из предиктивных вариантов соттветсвует нажатым кавишам, и вывод его на экран. Если нажата клавиша соотвествющая стролочкам вверх или вниз то происходит переключение между вариантами слов. при повторном нажатии на выбраном слове робела или шифт происходит вставка слова в текстовое поле.
Если зажат пробел и нажата другая кнопка то буква печатается с заглавной
*/
func БуферНажатий(СобытиеКлавиатуры *ДанныеКлавиатурногоСобытия) {
	// Если нажата клавиша пробел 1 раз то это пробел,если пробел нажат два раза подрят или зажат то вставляем код shift и следующая буква бует заглавная
	// ПечатьТекста(слово)
	// var состояниеКлавишы [256]byte
	// ИнфоБФ("СобытиеКлавиатуры %+v \n", СобытиеКлавиатуры)
	// ПолучитьСостояниеКлавиатуры.Call(uintptr(unsafe.Pointer(&состояниеКлавишы)))
	// ИнфоБФ("состояниеКлавишы %+v \n", состояниеКлавишы)ВАЙЦ
	// ЗажатМодификатор := win.GetAsyncKeyState(co.VK_SPACE)
	// Инфо("ЗажатМодификатор %+v \n", ЗажатМодификатор)

	if СобытиеКлавиатуры.ТипСобытия == win.WPARAM(co.WM_KEYDOWN) && ЗажатаяКлавиша[true] != СобытиеКлавиатуры.ВиртуальныйКод {
		// Сохраним клавишу которая была нажата в качестве зажатой
		ЗажатаяКлавиша[true] = СобытиеКлавиатуры.ВиртуальныйКод

		switch co.VK(СобытиеКлавиатуры.ВиртуальныйКод) {
		case co.VK_SPACE:
			// ЗажатаяКлавиша[СобытиеКлавиатуры.ВиртуальныйКод] = true

			// Выведем первое слово из предложеных вариантов
			// и начнём накапливать массив для новго слова
		// case co.VK_SHIFT:
		// ЗажатаяКлавиша[СобытиеКлавиатуры.ВиртуальныйКод] = true
		// продолжаем накапливать массив слов, но с заглавной буквы
		case co.VK_SHIFT:

			// Инфо("ЗажатМодификатор VK_SHIFT %+v \n", ВиртуальныйКод(ЗажатМодификатор))

		case co.VK_LCONTROL:
			// Инфо("ЗажатМодификатор VK_LCONTROL %+v \n", ВиртуальныйКод(ЗажатМодификатор))

		default:
			// ЗажатМодификатор := win.GetAsyncKeyState(co.VK_SPACE)
			// if ЗажатМодификатор != 0 {
			// 	Буфер.ТекущееСлово = append(Буфер.ТекущееСлово, ВиртуальныйКод(Shift))
			// }
			// Буфер.ТекущееСлово = append(Буфер.ТекущееСлово, СобытиеКлавиатуры.ВиртуальныйКод)
			// продолжаем накапливать массив слов

		}

	}

	if СобытиеКлавиатуры.ТипСобытия == win.WPARAM(co.WM_KEYUP) {
		// ЗажатаяКлавиша[true] = 0
		var ЗажатМодификатор uint16
		// Если следующий код отличается от послднего зажатого то получим в каком сотоянии находится зажатая клавиша
		if ЗажатаяКлавиша[true] > 0 && ЗажатаяКлавиша[true] != СобытиеКлавиатуры.ВиртуальныйКод {
			ЗажатМодификатор = win.GetAsyncKeyState(co.VK(ЗажатаяКлавиша[true]))
		}
		Инфо("СобытиеКлавиатуры.ВиртуальныйКод %+v \n", СобытиеКлавиатуры.ВиртуальныйКод)

		switch co.VK(СобытиеКлавиатуры.ВиртуальныйКод) {
		case co.VK_SPACE: // AiCopilot-Code начало: Обработка пробела
			if len(ТекущиеПредсказания) > 0 {
				словоДляВставки := ТекущиеПредсказания[ВыбранноеПредсказаниеИндекс]
				// Удаляем набранные символы
				количествоУдаляемыхСимволов := len(Буфер.ТекущееСлово)
				for i := 0; i < количествоУдаляемыхСимволов; i++ {
					ПечатьБуквы(Backspace, true)  // Нажатие Backspace
					ПечатьБуквы(Backspace, false) // Отпускание Backspace
				}
				ПечатьТекста(словоДляВставки)
				КэшЧастотности.УвеличитьЧастотность(словоДляВставки) // Обновляем частотность
			}
			Буфер.ТекущееСлово = []ВиртуальныйКод{}
			ТекущиеПредсказания = []string{}
			ВыбранноеПредсказаниеИндекс = 0
			if HWNDГлавногоОкна != 0 {
				HWNDГлавногоОкна.SendMessage(WM_UPDATE_PREDICTIONS, 0, 0)
			}
		case co.VK_UP: // AiCopilot-Code начало: Обработка стрелки вверх
			if len(ТекущиеПредсказания) > 0 {
				ВыбранноеПредсказаниеИндекс--
				if ВыбранноеПредсказаниеИндекс < 0 {
					ВыбранноеПредсказаниеИндекс = len(ТекущиеПредсказания) - 1
				}
				if HWNDГлавногоОкна != 0 {
					HWNDГлавногоОкна.SendMessage(WM_UPDATE_PREDICTIONS, 0, 0)
				}
			}
		case co.VK_DOWN: // AiCopilot-Code начало: Обработка стрелки вниз
			if len(ТекущиеПредсказания) > 0 {
				ВыбранноеПредсказаниеИндекс++
				if ВыбранноеПредсказаниеИндекс >= len(ТекущиеПредсказания) {
					ВыбранноеПредсказаниеИндекс = 0
				}
				if HWNDГлавногоОкна != 0 {
					HWNDГлавногоОкна.SendMessage(WM_UPDATE_PREDICTIONS, 0, 0)
				}
			}
		case co.VK_BACK: // AiCopilot-Code начало: Обработка Backspace
			if len(Буфер.ТекущееСлово) > 0 {
				Буфер.ТекущееСлово = Буфер.ТекущееСлово[:len(Буфер.ТекущееСлово)-1]
				каналПредсказания <- Буфер.ТекущееСлово // Отправляем обновленный буфер
			} else {
				// Если буфер пуст, очищаем предсказания
				ТекущиеПредсказания = []string{}
				ВыбранноеПредсказаниеИндекс = 0
				if HWNDГлавногоОкна != 0 {
					HWNDГлавногоОкна.SendMessage(WM_UPDATE_PREDICTIONS, 0, 0)
				}
			}
		case co.VK_SHIFT, co.VK_LSHIFT, co.VK_RSHIFT, co.VK_CONTROL, co.VK_LCONTROL, co.VK_RCONTROL, co.VK_MENU, co.VK_LMENU, co.VK_RMENU: // AiCopilot-Code начало: Игнорируем клавиши-модификаторы
			// Игнорируем клавиши-модификаторы при добавлении в буфер
			Инфо("Игнорируем клавишу-модификатор: 0x%X", СобытиеКлавиатуры.ВиртуальныйКод)
		default: // AiCopilot-Code конец: Обработка остальных клавиш
			// AiCopilot-Code начало
			// Перемещаем окна к позиции курсора при каждом нажатии
			ПереместитьОкнаКурсору()
			// AiCopilot-Code конец
			Инфо("ЗажатМодификатор 2%+v \n", ВиртуальныйКод(ЗажатМодификатор))
			Инфо("СобытиеКлавиатуры.ВиртуальныйКод %+v \n", СобытиеКлавиатуры.ВиртуальныйКод)
			Буфер.ТекущееСлово = append(Буфер.ТекущееСлово, СобытиеКлавиатуры.ВиртуальныйКод)
			каналПредсказания <- Буфер.ТекущееСлово // AiCopilot-Code начало: Отправляем весь буфер
		}
	}
}

// AiCopilot-Code начало: Реализация алгоритма предсказания слов
func ПредсказаниеСлов() {
	for коды := range каналПредсказания {
		ТекущиеПредсказания = ПредсказатьСлова(коды)
		if HWNDГлавногоОкна != 0 {
			HWNDГлавногоОкна.PostMessage(WM_UPDATE_PREDICTIONS, 0, 0)
		}
	}
}

// ПредсказатьСлова генерирует предсказания слов на основе последовательности нажатых клавиш.
// AiCopilot-Code начало
func ПредсказатьСлова(коды []ВиртуальныйКод) []string {
	if len(коды) == 0 {
		return []string{}
	}

	возможныеПрефиксы := []string{""}

	for _, код := range коды {
		новыеПрефиксы := []string{}
		кнопка, ок := Алвафит[код]
		if !ок {
			// Если код клавиши не найден в Алвафит, игнорируем его
			return []string{}
		}

		возможныеБуквы := []string{}
		возможныеБуквы = append(возможныеБуквы, кнопка.буквы["ру"]...)
		возможныеБуквы = append(возможныеБуквы, кнопка.буквы["en"]...)

		for _, префикс := range возможныеПрефиксы {
			for _, буква := range возможныеБуквы {
				новыйПрефикс := префикс + strings.ToLower(буква) // Приводим к нижнему регистру для сравнения
				if Словари.ЯвляетсяПрефиксом(новыйПрефикс) {
					новыеПрефиксы = append(новыеПрефиксы, новыйПрефикс)
				}
			}
		}
		возможныеПрефиксы = новыеПрефиксы
	}

	const максПредсказаний = 20
	предсказанныеСлова := []string{}
	виденные := make(map[string]bool)
	for _, префикс := range возможныеПрефиксы {
		слова := Словари.трие.НайтиСловаСПрефиксом(префикс, максПредсказаний)
		for _, слово := range слова {
			if !виденные[слово] && len(предсказанныеСлова) < максПредсказаний {
				виденные[слово] = true
				предсказанныеСлова = append(предсказанныеСлова, слово)
			}
		}
	}

	// Ранжирование слов
	sort.Slice(предсказанныеСлова, func(i, j int) bool {
		freqI := КэшЧастотности.ПолучитьЧастотность(предсказанныеСлова[i])
		freqJ := КэшЧастотности.ПолучитьЧастотность(предсказанныеСлова[j])

		if freqI != freqJ {
			return freqI > freqJ // Сначала по частотности (убывание)
		}
		if len(предсказанныеСлова[i]) != len(предсказанныеСлова[j]) {
			return len(предсказанныеСлова[i]) > len(предсказанныеСлова[j]) // Затем по длине (убывание)
		}
		return предсказанныеСлова[i] < предсказанныеСлова[j] // Затем по алфавиту (возрастание)
	})

	return предсказанныеСлова
}

// AiCopilot-Code конец

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
func ПечатьБуквы(code ВиртуальныйКод, press bool) {
	Инфо(" ПечатьБуквы %+v press %+v \n", code, press)

	var direction uint32
	if !press {
		direction = KEYEVENTF_KEYUP
	}
	inputs := INPUT{
		Type: INPUT_KEYBOARD,
		Ki: KEYBDINPUT{
			WVk:         uint16(code),
			WScan:       0,
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

	inputs := make([]INPUT, 0, len(букваДляВывода)*2)

	for _, руна := range букваДляВывода {
		inputs = append(inputs, INPUT{
			Type: INPUT_KEYBOARD,
			Ki: KEYBDINPUT{
				WScan:       uint16(руна),
				DwFlags:     KEYEVENTF_UNICODE,
				DwExtraInfo: УказательНаПоток,
			}})
		inputs = append(inputs, INPUT{
			Type: INPUT_KEYBOARD,
			Ki: KEYBDINPUT{
				WScan:       uint16(руна),
				DwFlags:     KEYEVENTF_UNICODE | KEYEVENTF_KEYUP,
				DwExtraInfo: УказательНаПоток,
			}})
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
		ПозицияОконПриСтарте(ДО)

		// Сохраняем HWND и устанавливаем перехватчик сообщений для обновления предсказаний
		if HWNDГлавногоОкна == 0 {
			HWNDГлавногоОкна = ДО
			оригинальнаяПроцедураГлавногоОкна = ДО.SetWindowLongPtr(co.GWLP_WNDPROC, syscall.NewCallback(новаяПроцедураГлавногоОкна))
		}
	})

}

// AiCopilot-Code начало
// RECT определяет прямоугольник по координатам его верхнего левого и нижнего правого углов.
type RECT struct {
	Left, Top, Right, Bottom int32
}

// GUITHREADINFO содержит информацию о потоке графического интерфейса.
type GUITHREADINFO struct {
	CbSize        uint32
	Flags         uint32
	HwndActive    win.HWND
	HwndFocus     win.HWND
	HwndCapture   win.HWND
	HwndMenuOwner win.HWND
	HwndMoveSize  win.HWND
	HwndCaret     win.HWND
	RcCaret       RECT
}

// ПолучитьПозициюКурсора находит позицию системного текстового курсора (каретки) с помощью GetGUIThreadInfo.
// Этот метод более надежен, чем AttachThreadInput+GetCaretPos, так как он не требует "присоединения"
// к другому потоку и лучше работает с разными уровнями прав доступа приложений.
func ПолучитьПозициюКурсора() (win.POINT, bool) {
	var точка win.POINT

	// Получаем ID потока для активного в данный момент окна.
	потокID, _, _ := GetWindowThreadProcessId.Call(uintptr(win.GetForegroundWindow()), 0)
	if потокID == 0 {
		Инфо("ПолучитьПозициюКурсора: Не удалось получить ID потока активного окна.")
		return точка, false
	}

	var guiInfo GUITHREADINFO
	guiInfo.CbSize = uint32(unsafe.Sizeof(guiInfo))

	// Запрашиваем информацию о потоке GUI.
	успех, _, err := GetGUIThreadInfo.Call(потокID, uintptr(unsafe.Pointer(&guiInfo)))
	if успех == 0 {
		Инфо("ПолучитьПозициюКурсора: GetGUIThreadInfo не удалось. Ошибка: %v", err)
		return точка, false
	}

	// Проверяем, есть ли у потока видимая каретка.
	// rcCaret будет (0,0,0,0) если каретки нет.
	if guiInfo.HwndCaret == 0 {
		Инфо("ПолучитьПозициюКурсора: Каретка не найдена (HwndCaret is null).")
		return точка, false
	}

	// RcCaret возвращает координаты в клиентских координатах окна HwndCaret.
	// Конвертируем их в экранные координаты.
	точка.X = guiInfo.RcCaret.Left
	точка.Y = guiInfo.RcCaret.Bottom
	guiInfo.HwndCaret.ClientToScreenPt(&точка)

	Инфо("ПолучитьПозициюКурсора: Найдена позиция каретки: X=%d, Y=%d", точка.X, точка.Y)

	// Дополнительная проверка: если координаты нулевые, возможно, каретка просто скрыта.
	if точка.X == 0 && точка.Y == 0 {
		Инфо("ПолучитьПозициюКурсора: Позиция каретки (0,0), возможно, она скрыта.")
		return точка, false
	}

	return точка, true
}

// AiCopilot-Code конец

// AiCopilot-Code начало
// ПереместитьОкнаКурсору перемещает оба окна (основное и подсказок) к текущей позиции текстового курсора.
// Если курсор не найден, функция ничего не делает, оставляя окна на их текущих местах.
func ПереместитьОкнаКурсору() {
	if курсор, ок := ПолучитьПозициюКурсора(); ок {
		posX := курсор.X
		posY := курсор.Y + 25 // Смещение под курсором

		положение := int32(-1) // HWND_TOPMOST

		if ОсновноеОкноПрограммы != nil {
			основноеОкно := ОсновноеОкноПрограммы.окно.Hwnd()
			основноеОкно.SetWindowPos(win.HWND(uintptr(положение)), posX, posY, 0, 0, co.SWP_NOSIZE|co.SWP_ASYNCWINDOWPOS|co.SWP_NOACTIVATE)
		}

		if ОсновноеОкноПодсказок != nil {
			// Позиционируем окно подсказок справа от основного окна
			posXПодсказок := posX + 310
			posYПодсказок := posY
			ОсновноеОкноПодсказок.hwnd.SetWindowPos(win.HWND(uintptr(положение)), posXПодсказок, posYПодсказок, 0, 0, co.SWP_NOSIZE|co.SWP_ASYNCWINDOWPOS|co.SWP_NOACTIVATE)
		}
	}
}

// AiCopilot-Code конец

// ПозицияОконПриСтарте задает начальное положение для окон при первом их появлении.
// Пытается разместить у курсора, а если не вышло - в левом нижнем углу экрана.
func ПозицияОконПриСтарте(ДО win.HWND) {
	var posX, posY int32

	if курсор, ок := ПолучитьПозициюКурсора(); ок {
		// Если удалось получить позицию текстового курсора, используем ее.
		posX = курсор.X
		posY = курсор.Y + 25 // Небольшое смещение вниз, чтобы окно появилось под курсором.
	} else {
		// В противном случае (если курсор не найден), используем старую логику:
		// позиционирование в левом нижнем углу экрана.
		высотаЭкрана := int32(win.GetSystemMetrics(co.SM_CYSCREEN))
		posX = 50
		posY = высотаЭкрана - 350 // Примерно внизу экрана, норана - 350
	}

	Инфо("Начальная позиция окна: X=%d, Y=%d\n", posX, posY)
	положение := int32(-1) // HWND_TOPMOST, чтобы окно было поверх других.

	// Устанавливаем позицию основного окна
	ДО.SetWindowPos(win.HWND(uintptr(положение)), posX, posY, 0, 0, co.SWP_SHOWWINDOW|co.SWP_NOSIZE|co.SWP_ASYNCWINDOWPOS|co.SWP_NOACTIVATE)

	// Устанавливаем позицию окна подсказок
	if ОсновноеОкноПодсказок != nil {
		posXПодсказок := posX + 310
		posYПодсказок := posY
		ОсновноеОкноПодсказок.hwnd.SetWindowPos(win.HWND(uintptr(положение)), posXПодсказок, posYПодсказок, 0, 0, co.SWP_SHOWWINDOW|co.SWP_NOSIZE|co.SWP_ASYNCWINDOWPOS|co.SWP_NOACTIVATE)
	}
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

var (
	registerClassExW = user32.NewProc("RegisterClassExW")
	createWindowExW  = user32.NewProc("CreateWindowExW")
	defWindowProcW   = user32.NewProc("DefWindowProcW")
	setWindowTextW   = user32.NewProc("SetWindowTextW")
)

type WNDCLASSEX struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

func оконнаяПроцедураПодсказок(hwnd win.HWND, msg uint32, wParam win.WPARAM, lParam win.LPARAM) uintptr {
	switch msg {
	case 0x0139: // WM_CTLCOLORSTATIC
		hdc := win.HDC(wParam)
		hwndControl := win.HWND(lParam)

		for i, hChild := range ОсновноеОкноПодсказок.дети {
			if hChild == hwndControl {
				if i == ВыбранноеПредсказаниеИндекс && i < len(ОсновноеОкноПодсказок.тексты) {
					hdc.SetBkColor(win.RGB(255, 0, 123))
					SetTextColor.Call(uintptr(hdc), uintptr(win.RGB(139, 234, 0)))
					return uintptr(win.CreateSolidBrush(win.RGB(255, 0, 123)))
				} else {
					hdc.SetBkColor(win.RGB(29, 13, 41))
					SetTextColor.Call(uintptr(hdc), uintptr(win.RGB(255, 255, 255)))
					return uintptr(win.CreateSolidBrush(win.RGB(29, 13, 41)))
				}
			}
		}
	}
	ret, _, _ := defWindowProcW.Call(uintptr(hwnd), uintptr(msg), uintptr(wParam), uintptr(lParam))
	return ret
}

func НовоеОкноПодсказок() {
	hInst := uintptr(win.GetModuleHandle(win.StrOptNone()))

	className := syscall.StringToUTF16Ptr("ПотокПодсказки")

	кисть := win.CreateSolidBrush(win.RGB(63, 39, 81))

	wcx := WNDCLASSEX{
		cbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		style:         uint32(co.CS_HREDRAW | co.CS_VREDRAW),
		lpfnWndProc:   syscall.NewCallback(оконнаяПроцедураПодсказок),
		hInstance:     hInst,
		hbrBackground: uintptr(кисть),
		lpszClassName: className,
	}
	registerClassExW.Call(uintptr(unsafe.Pointer(&wcx)))

	hwndRet, _, _ := createWindowExW.Call(
		uintptr(co.WS_EX_TOOLWINDOW|co.WS_EX_NOACTIVATE|co.WS_EX_TOPMOST|co.WS_EX_LAYERED),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("ПотоК"))),
		uintptr(co.WS_POPUP),
		0, 0, 300, 50,
		0, 0, hInst, 0,
	)
	hwnd := win.HWND(hwndRet)
	hwnd.SetLayeredWindowAttributes(0, 190, 0x00000002)

	количество := 20
	дети := make([]win.HWND, количество)
	staticClass := syscall.StringToUTF16Ptr("STATIC")
	for i := 0; i < количество; i++ {
		hChildRet, _, _ := createWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(staticClass)),
			0,
			uintptr(co.WS_CHILD|co.WS_VISIBLE|co.WS_BORDER)|uintptr(co.SS_CENTER),
			uintptr(5), uintptr(int32(5+i*(20+5))), uintptr(290), uintptr(20),
			uintptr(hwnd), 0, hInst, 0,
		)
		дети[i] = win.HWND(hChildRet)
	}

	ОсновноеОкноПодсказок = &ОкноПодсказок{
		hwnd:   hwnd,
		дети:   дети,
		тексты: make([]string, 0),
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
		столбцы:       5,
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

// AiCopilot-Code начало: Новые структуры и функции для словарей и кэша частотности

// СловариСлов хранит словари русского и английского языков
type СловариСлов struct {
	русскийСловарь     map[string]bool
	английскийСловарь  map[string]bool
	русскийПрефиксы    map[string]bool
	английскийПрефиксы map[string]bool
	трие               *Трие
}

// НовыеСловариСлов создает новый экземпляр СловариСлов
func НовыеСловариСлов() *СловариСлов {
	return &СловариСлов{
		русскийСловарь:     make(map[string]bool),
		английскийСловарь:  make(map[string]bool),
		русскийПрефиксы:    make(map[string]bool),
		английскийПрефиксы: make(map[string]bool),
		трие:               НовыйТрие(),
	}
}

// ЗагрузитьСловари загружает слова из файлов в память
func (словари *СловариСлов) ЗагрузитьСловари(путьРусский, путьАнглийский string) QErrors.СтатусОтвета {
	статусРусский := словари.загрузитьСловарь(путьРусский, словари.русскийСловарь, словари.русскийПрефиксы)
	if статусРусский.Код != QErrors.Ок {
		return статусРусский
	}
	статусАнглийский := словари.загрузитьСловарь(путьАнглийский, словари.английскийСловарь, словари.английскийПрефиксы)
	if статусАнглийский.Код != QErrors.Ок {
		return статусАнглийский
	}
	Инфо("Словари успешно загружены.")
	return QErrors.СтатусОтвета{Код: QErrors.Ок, Текст: "Словари успешно загружены."}
}

func (словари *СловариСлов) загрузитьСловарь(путь string, словарь map[string]bool, префиксы map[string]bool) QErrors.СтатусОтвета {
	файл, err := os.Open(путь)
	if err != nil {
		return QErrors.СтатусОтвета{Код: QErrors.ОшибкаCBORДеКодирования, Текст: fmt.Sprintf("Не удалось открыть файл словаря %s: %v", путь, err)}
	}
	defer файл.Close()

	сканер := bufio.NewScanner(файл)
	for сканер.Scan() {
		слово := strings.TrimSpace(strings.ToLower(сканер.Text()))
		if len(слово) > 0 && !strings.HasPrefix(слово, "#") { // Игнорируем пустые строки и комментарии
			словарь[слово] = true
			// Добавляем все префиксы слова
			for i := 1; i <= len(слово); i++ {
				префиксы[слово[:i]] = true
			}
			// Добавляем слово в Trie для поиска по префиксам
			словари.трие.ДобавитьСлово(слово)
		}
	}

	if err := сканер.Err(); err != nil {
		return QErrors.СтатусОтвета{Код: QErrors.ОшибкаCBORДеКодирования, Текст: fmt.Sprintf("Ошибка чтения файла словаря %s: %")}
	}
	return QErrors.СтатусОтвета{Код: QErrors.Ок, Текст: "Словари успешно загружены."}

}

// ЯвляетсяСловом проверяет, является ли строка словом в любом из словарей
func (словари *СловариСлов) ЯвляетсяСловом(слово string) bool {
	return словари.русскийСловарь[слово] || словари.английскийСловарь[слово]
}

// ЯвляетсяПрефиксом проверяет, является ли строка префиксом слова в любом из словарей
func (словари *СловариСлов) ЯвляетсяПрефиксом(префикс string) bool {
	return словари.русскийПрефиксы[префикс] || словари.английскийПрефиксы[префикс]
}

// КэшЧастотностиСлов хранит частотность использования слов
type КэшЧастотностиСлов struct {
	частотность map[string]int
}

// НовыиКэшЧастотностиСлов создает новый экземпляр КэшЧастотностиСлов
func НовыиКэшЧастотностиСлов() *КэшЧастотностиСлов {
	return &КэшЧастотностиСлов{
		частотность: make(map[string]int),
	}
}

// ЗагрузитьКэш загружает кэш частотности из файла
func (кэш *КэшЧастотностиСлов) ЗагрузитьКэш(путь string) QErrors.СтатусОтвета {
	файл, err := os.ReadFile(путь)
	if err != nil {
		// Если файл не найден, это не ошибка, просто кэш пуст
		if os.IsNotExist(err) {
			Инфо("Файл кэша частотности не найден, создаем новый.")
			return QErrors.СтатусОтвета{Код: QErrors.Ок, Текст: "Словари успешно загружены."}
		}
		return QErrors.СтатусОтвета{Код: QErrors.Ок, Текст: fmt.Sprintf("Не удалось прочитать файл кэша частотности %s: %v", путь, err)}
	}

	err = json.Unmarshal(файл, &кэш.частотность)
	if err != nil {
		return QErrors.СтатусОтвета{Код: QErrors.ОшибкаДекодирования, Текст: fmt.Sprintf("Ошибка декодирования JSON кэша частотности: %s: %v", путь, err)}

	}
	Инфо("Кэш частотности успешно загружен.")
	return QErrors.СтатусОтвета{Код: QErrors.Ок, Текст: "Словари успешно загружены."}

}

// СохранитьКэш сохраняет кэш частотности в файл
func (кэш *КэшЧастотностиСлов) СохранитьКэш(путь string) QErrors.СтатусОтвета {
	данные, err := json.MarshalIndent(кэш.частотность, "", "  ")
	if err != nil {
		return QErrors.СтатусОтвета{Код: QErrors.ОшибкаКодирования, Текст: "Ошибка кодирования JSON кэша частотности."}

	}

	err = os.WriteFile(путь, данные, 0644)
	if err != nil {
		return QErrors.СтатусОтвета{Код: QErrors.ОшибкаОткрытияФайла, Текст: "Не удалось записать файл кэша частотности."}
	}
	Инфо("Кэш частотности успешно сохранен.")
	return QErrors.СтатусОтвета{Код: QErrors.Ок, Текст: "Кэш частотности успешно сохранен."}
}

// УвеличитьЧастотность увеличивает частотность слова
func (кэш *КэшЧастотностиСлов) УвеличитьЧастотность(слово string) {
	кэш.частотность[strings.ToLower(слово)]++
}

// ПолучитьЧастотность возвращает частотность слова
func (кэш *КэшЧастотностиСлов) ПолучитьЧастотность(слово string) int {
	return кэш.частотность[strings.ToLower(слово)]
}

// ОбновитьПредсказания обновляет отображение предсказанных слов в окне подсказок
func (окноПодсказок *ОкноПодсказок) ОбновитьПредсказания(предсказания []string, выбранныйИндекс int) {
	окноПодсказок.тексты = предсказания

	for i, hChild := range окноПодсказок.дети {
		if i < len(предсказания) {
			hChild.SetWindowText(предсказания[i])
			hChild.ShowWindow(co.SW_SHOW)
			hChild.InvalidateRect(nil, true)
		} else {
			hChild.ShowWindow(co.SW_HIDE)
		}
	}

	высотаЭлемента := int32(20)
	отступ := int32(5)
	if len(предсказания) == 0 {
		окноПодсказок.hwnd.SetWindowPos(0, 0, 0, 300, 50, co.SWP_NOMOVE|co.SWP_NOZORDER)
	} else {
		новаяВысота := int32(len(предсказания))*высотаЭлемента + int32(len(предсказания)+1)*отступ
		if новаяВысота < 50 {
			новаяВысота = 50
		}
		окноПодсказок.hwnd.SetWindowPos(0, 0, 0, 300, новаяВысота, co.SWP_NOMOVE|co.SWP_NOZORDER)

		for i, hChild := range окноПодсказок.дети[:len(предсказания)] {
			y := отступ + int32(i)*(высотаЭлемента+отступ)
			hChild.MoveWindow(отступ, y, 300-2*отступ, высотаЭлемента, true)
		}
	}
}
