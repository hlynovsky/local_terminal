<?php
require(__DIR__.'/../../config.php');

$id = required_param('id', PARAM_INT); // course_module ID

$cm = get_coursemodule_from_id('terminal', $id, 0, false, MUST_EXIST);
$course = $DB->get_record('course', ['id' => $cm->course], '*', MUST_EXIST);
$terminal = $DB->get_record('terminal', ['id' => $cm->instance], '*', MUST_EXIST);

require_login($course, true, $cm);

$PAGE->set_url('/mod/terminal/view.php', ['id' => $id]);
$PAGE->set_title(format_string($terminal->name));
$PAGE->set_heading(format_string($course->fullname));

// URL для иконки
$plugin_icon_url = "https://upload.wikimedia.org/wikipedia/commons/thumb/b/b3/Terminalicon2.png/640px-Terminalicon2.png";

echo $OUTPUT->header();

if (!empty($terminal->intro)) {
    echo $OUTPUT->box(format_module_intro('terminal', $terminal, $cm->id), 'generalbox mod_introbox', 'terminalintro');
}


header('Content-Type: text/html; charset=utf-8');
?>
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>Terminal</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/xterm/css/xterm.css" />
  <style>
    /* Основной фон, цвета, чтобы соответствовать стилю Moodle */
    body {
      margin: 0;
      background-color: #f4f4f4; /* Светлый фон для Moodle */
      color: #333; /* Тёмный текст для читаемости */
      font-family: "Arial", sans-serif;
      display: flex;
      justify-content: center;
      align-items: center;
      height: 100vh;
    }

    /* Стили для контейнера терминала */
    #terminal-container {
      resize: both;
      overflow: auto;
      border: 2px solid #ddd; /* Светлый цвет границы */
      width: 800px;
      height: 500px;
      min-width: 300px;
      min-height: 200px;
      background-color: #fff; /* Белый фон внутри терминала */
    }

    /* Стили для самого терминала */
    #terminal {
      width: 100%;
      height: 100%;
    }

    /* Дополнительные стили для скроллинга и адаптивности */
    .xterm-viewport {
      background-color: #fff; /* Белый фон для области вывода */
    }

    /* Кастомизация для цветовой схемы терминала */
    .xterm {
      background-color: #fafafa; /* Светлый фон для терминала */
      color: #333; /* Темный текст для читаемости */
    }

    /* Настройка подсветки цветов терминала */
    .xterm-foreground {
      color: #333;
    }

    .xterm-background {
      background-color: #fafafa;
    }
  </style>
</head>
<body>
  <div id="terminal-container">
    <div id="terminal"></div>
  </div>

  <script src="https://cdn.jsdelivr.net/npm/xterm/lib/xterm.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/xterm-addon-fit/lib/xterm-addon-fit.js"></script>
  <script>
    const term = new Terminal({
      theme: {
        background: '#fafafa',  // Светлый фон для терминала
        foreground: '#333',     // Тёмный текст для читаемости
        cursor: '#333',        // Тёмный курсор
        selectionBackground: '#3399ff', // Цвет выделения
        black: '#000000',      // Черный
        red: '#ff0000',        // Красный
        green: '#00ff00',      // Зеленый
        yellow: '#ffff00',     // Желтый
        blue: '#0000ff',       // Синий
        magenta: '#ff00ff',    // Магента
        cyan: '#00ffff',       // Голубой
        white: '#ffffff',      // Белый
        brightBlack: '#666666',// Яркий черный
        brightRed: '#ff6666',  // Ярко-красный
        brightGreen: '#66ff66',// Ярко-зеленый
        brightYellow: '#ffff66', // Ярко-желтый
        brightBlue: '#6666ff', // Ярко-синий
        brightMagenta: '#ff66ff', // Яркая магента
        brightCyan: '#66ffff', // Яркий голубой
        brightWhite: '#ffffff' // Яркий белый
      }
    });

    const fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);

    term.open(document.getElementById('terminal'));
    fitAddon.fit(); // первый раз подогнать

    const userID = "<?= $USER->username ?>";
    const ws = new WebSocket("ws://83.220.170.151:8765/terminal");

    let terminalReady = false;

    ws.onopen = () => {
      ws.send(JSON.stringify({ user_id: userID }));
    };

    ws.onmessage = (event) => {
      const data = event.data;

      try {
        const parsed = JSON.parse(data);
        if (parsed.status === "ready") {
          terminalReady = true;
          term.focus();
          return;
        }
      } catch (_) {}

      const clean = stripAnsiArtifacts(data);
      term.write(clean);
    };

    term.onData((data) => {
      if (terminalReady) {
        ws.send(data);
      }
    });

    function stripAnsiArtifacts(text) {
      text = text.replace(/\x1b\]0;.*?\x07/g, '');
      text = text.replace(/\x1b\[\?2004[hl]/g, '');
      return text;
    }

    ws.onclose = () => {
      term.write('\r\n🔒 Соединение закрыто\r\n');
    };

    ws.onerror = (err) => {
      term.write(`\r\n❌ WebSocket error: ${err.message || err}\r\n`);
    };

    const container = document.getElementById('terminal-container');
    const resizeObserver = new ResizeObserver(() => {
      fitAddon.fit();
    });
    resizeObserver.observe(container);
  </script>
</body>
</html>

<?php
echo $OUTPUT->footer();
