<?php
function terminal_supports($feature) {
    switch($feature) {
        case FEATURE_MOD_INTRO: return true;
        case FEATURE_SHOW_DESCRIPTION: return true;
        default: return null;
    }
}
function terminal_add_instance($data) {
    global $DB;
    $data->timecreated = time();
    $data->timemodified = time();
    return $DB->insert_record('terminal', $data);
}

function terminal_update_instance($data) {
    global $DB;
    $data->timemodified = time();
    $data->id = $data->instance;
    return $DB->update_record('terminal', $data);
}

