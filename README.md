# ReactorOpt

Учебное настольное приложение для оптимизации функций двух переменных.
Go, Wails v3, Vue 3, TypeScript и Plotly.

Методы: Нелдера — Мида, BFGS и CMA-ES.
Есть ограничения, библиотека задач, учётные записи и графики 2D/3D.

[Скриншоты приложения](docs/screenshots/)

## Запуск из исходников

Нужны Go 1.27.4, Node.js 24 и [системные зависимости Wails](https://v3.wails.io/getting-started/installation/).

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0
npm --prefix frontend ci
wails3 doctor
wails3 dev
```

Сборка: `wails3 build`. Результат находится в `bin/`.
При первом запуске создайте администратора. Каталог данных можно задать через `--data-dir`.
