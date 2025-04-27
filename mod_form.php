<?php
require_once($CFG->dirroot.'/course/moodleform_mod.php');

class mod_terminal_mod_form extends moodleform_mod {
    public function definition() {
        $mform = $this->_form;

        $this->standard_coursemodule_elements(); // включает name, visible, groupmode и др.
        $this->standard_intro_elements();        // intro и introformat
        $this->add_action_buttons();             // кнопки сохранения
    }
}
