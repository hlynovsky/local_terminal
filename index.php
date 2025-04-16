<?php
require_once(__DIR__.'/../../config.php');
require_once($CFG->libdir.'/adminlib.php');

require_login();
$context = context_system::instance();
require_capability('moodle/site:config', $context);

$PAGE->set_url('/local/terminal/index.php');
$PAGE->set_context($context);
$PAGE->set_title(get_string('terminal', 'local_terminal'));
$PAGE->set_heading(get_string('terminal', 'local_terminal'));
$PAGE->requires->js_call_amd('local_terminal/terminal', 'init');

$token = bin2hex(random_bytes(16));
$SESSION->terminal_token = $token;

echo $OUTPUT->header();

echo $OUTPUT->render_from_template('local_terminal/terminal', [
    'service_url' => 'ws://localhost:8080/ws',
    'token' => $token,
    'userid' => $USER->id
]);

echo $OUTPUT->footer();