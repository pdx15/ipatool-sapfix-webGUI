# Размещение sapsigner.exe в репозитории (встроенная копия)

> ⚠️ Этот файл — инструкция для владельца **приватного** репозитория.
> Бинарники `sapsigner.exe` и `libunicorn.dll` принадлежат **AltStore** и Apple,
> поэтому в **публичном** репозитории их размещать нельзя (нарушение лицензий).
> В приватном репозитории — на ваш страх и риск.

## Что нужно положить

`ipatool` на Windows просто запускает `sapsigner.exe` как внешний процесс и
запускает его из его же папки (`cmd.Dir = filepath.Dir(path)`). Поэтому рядом с
`sapsigner.exe` должны лежать все его зависимости — они уже добавлены в
приватную сборку (коммиты владельца репозитория) в папку `tools\`:

Нужны **4 элемента**:

| Файл / папка       | Зачем                                                        |
|--------------------|--------------------------------------------------------------|
| `sapsigner.exe`    | сам подписант (выполняет SAP-подпись)                        |
| `libunicorn.dll`   | CPU-эмулятор Unicorn, на котором он построен                  |
| `ucworker.dll`     | рабочий поток эмулятора                                      |
| `sap-cache/`       | Apple-фреймворки (`CommerceKit`, `CommerceCore`, `CoreFP`, `CoreFP.icxs`, `storeagent`) |

`anisette.exe` и `ipatool.exe` из того же набора apple-tools **не нужны** — это
другие инструменты, для SAP-подписи они не используются.

## Куда положить

Положите эти 4 элемента **в одну папку**. Поддерживаются два варианта:

1. **Рядом с `ipatool.exe`** (та же папка).
2. **В подпапке `tools\`** рядом с `ipatool.exe` (или в текущей рабочей папке).

Пример структуры репозитория:

```
ipatool.exe
sapsigner.exe
libunicorn.dll
ucworker.dll
sap-cache/
  CommerceCore
  CommerceKit
  CoreFP
  CoreFP.icxs
  storeagent
```

или

```
ipatool.exe
tools/
  sapsigner.exe
  libunicorn.dll
  ucworker.dll
  sap-cache/
    ...
```

## Как это находится программой

Порядок поиска `sapsigner.exe` (см. `pkg/mescal/sapsigner_windows.go`):

1. Переменная окружения `IPATOOL_SAPSIGNER` (полный путь) — приоритет.
2. `SetSapsignerPath(...)` из кода.
3. **Встроенная копия**: папка рядом с `ipatool.exe` → `sapsigner.exe`, затем
   `ipatool.exe`-папка`\tools\sapsigner.exe`; затем то же для текущей рабочей папки.

Если встроенная копия найдена (рядом с `ipatool.exe` или в `tools\`), внешние
установки не требуются.

## Примечания

- Важно, чтобы `sap-cache`, `libunicorn.dll` и `ucworker.dll` лежали **в одной
  папке с `sapsigner.exe`** — код запускает его с рабочей директорией
  `filepath.Dir(path)`, чтобы зависимости подхватились.
- Размер встраиваемых файлов: `sapsigner.exe` ~10 МБ + `libunicorn.dll` ~21 МБ +
  `ucworker.dll` ~54 КБ + `sap-cache` ~38 МБ. Учитывайте лимиты Git-репозитория.
- Для кросс-сборки на CI эти файлы не компилируются — их нужно класть рядом с
  итоговым `ipatool.exe` на этапе упаковки релиза (например, в workflow-шаг).
