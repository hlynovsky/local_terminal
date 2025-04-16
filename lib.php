<?php
function local_terminal_get_k8s_config() {
    return [
        'namespace' => 'moodle-terminal',
        'image' => 'ubuntu:latest',
        'service_account' => 'moodle-terminal'
    ];
}

function local_terminal_create_pod($userid) {
    $plugin_path = __DIR__;
    $template = file_get_contents("$plugin_path/k8s/pod-template.yaml");
    
    $config = local_terminal_get_k8s_config();
    $placeholders = [
        '{{USER_ID}}' => $userid,
        '{{NAMESPACE}}' => $config['namespace'],
        '{{IMAGE}}' => $config['image']
    ];
    
    return str_replace(
        array_keys($placeholders),
        array_values($placeholders),
        $template
    );
}