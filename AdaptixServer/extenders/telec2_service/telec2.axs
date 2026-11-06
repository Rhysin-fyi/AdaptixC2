var telec2Dock = null;
var telec2Connections = null;
var telec2Console = null;
var telec2ConsoleTitle = null;
var telec2Health = null;
var telec2Destination = null;
var telec2File = null;
var telec2MaxBytes = 20 * 1024 * 1024;
var telec2Active = "";
var telec2Items = [];
var telec2Info = {};
var telec2Transcripts = {};

function telec2_time() {
    return new Date().toLocaleTimeString();
}

function telec2_seen(value) {
    if (!value) {
        return "Never";
    }
    let parsed = new Date(value);
    if (isNaN(parsed.getTime())) {
        return value;
    }
    return parsed.toLocaleString();
}

function telec2_append(destination, message) {
    if (!destination) {
        destination = telec2Active;
    }
    if (!destination) {
        return;
    }
    if (!telec2Transcripts[destination]) {
        telec2Transcripts[destination] = [];
    }
    let lines = telec2Transcripts[destination];
    lines.push("[" + telec2_time() + "] " + message);
    if (lines.length > 300) {
        lines.shift();
    }
    if (destination === telec2Active && telec2Console !== null) {
        telec2Console.setText(lines.join("\n"));
    }
}

function telec2_find_item(name) {
    for (let i = 0; i < telec2Items.length; i++) {
        if (telec2Items[i].name === name) {
            return i;
        }
    }
    return -1;
}

function telec2_find_table_row(name) {
    for (let row = 0; row < telec2Connections.rowCount(); row++) {
        if (telec2Connections.text(row, 0) === name) {
            return row;
        }
    }
    return -1;
}

function telec2_select(name) {
    if (!name || !telec2Info[name]) {
        return;
    }
    telec2Active = name;
    let info = telec2Info[name];
    telec2ConsoleTitle.setText(name + "  |  " + info.platform.toUpperCase() + "  |  @" + info.bot_username + "  |  " + info.state);
    telec2Health.setText(
        "Connection: " + name + "\n" +
        "Platform: " + info.platform + "\n" +
        "TeleC2 bot: @" + info.bot_username + " (" + info.bot_id + ")\n" +
        "Telegram group: " + info.chat_id + "\n" +
        "State: " + info.state + "\n" +
        "Last TeleC2 reply: " + telec2_seen(info.last_seen) + "\n\n" +
        "Verify checks the dedicated operator bot token and group access.\n" +
        "Status sends an addressed TeleC2 status request and marks this connection online only after a real bot reply."
    );
    telec2Console.setText((telec2Transcripts[name] || []).join("\n"));
    let index = telec2_find_item(name);
    if (index >= 0 && telec2Destination.currentIndex() !== index) {
        telec2Destination.setCurrentIndex(index);
    }
}

function telec2_active_or_message() {
    if (!telec2Active) {
        ax.show_message("TeleC2", "Select a TeleC2 connection first.");
        return "";
    }
    return telec2Active;
}

function telec2_verify() {
    let destination = telec2_active_or_message();
    if (!destination) {
        return;
    }
    telec2_append(destination, "Verifying operator bot and Telegram group...");
    ax.service_command("TeleC2", "probe", {destination: destination});
}

function telec2_status() {
    let destination = telec2_active_or_message();
    if (!destination) {
        return;
    }
    telec2_append(destination, "Requesting TeleC2 status...");
    ax.service_command("TeleC2", "status", {destination: destination});
}

function InitService() {
    telec2Connections = form.create_table(["Connection", "Platform", "Bot", "State", "Last Seen"]);
    telec2Connections.setSortingEnabled(true);
    telec2Connections.resizeToContent(0);
    telec2Connections.resizeToContent(1);
    telec2Connections.resizeToContent(3);

    let reloadButton = form.create_button("Reload Config");
    let verifyButton = form.create_button("Verify");
    let statusButton = form.create_button("Status");
    let connectionHint = form.create_label("Select a TeleC2 connection. Double-clicking verifies it.");

    let connectionPanel = form.create_panel();
    let connectionLayout = form.create_gridlayout();
    connectionLayout.addWidget(telec2Connections, 0, 0, 1, 5);
    connectionLayout.addWidget(reloadButton, 1, 0, 1, 1);
    connectionLayout.addWidget(verifyButton, 1, 1, 1, 1);
    connectionLayout.addWidget(statusButton, 1, 2, 1, 1);
    connectionLayout.addWidget(connectionHint, 1, 3, 1, 2);
    connectionPanel.setLayout(connectionLayout);

    telec2ConsoleTitle = form.create_label("No TeleC2 connection selected");
    telec2Console = form.create_textmulti();
    telec2Console.setReadOnly(true);
    telec2Console.setPlaceholder("TeleC2 replies and connection events appear here...");
    let consoleVerifyButton = form.create_button("Verify Connection");
    let consoleStatusButton = form.create_button("Request Status");
    let consoleClearButton = form.create_button("Clear View");
    let consoleSafety = form.create_label("Operational console: connection status and bot replies. Remote execution is not exposed.");
    let consolePanel = form.create_panel();
    let consoleLayout = form.create_gridlayout();
    consoleLayout.addWidget(telec2ConsoleTitle, 0, 0, 1, 4);
    consoleLayout.addWidget(telec2Console, 1, 0, 1, 4);
    consoleLayout.addWidget(consoleVerifyButton, 2, 0, 1, 1);
    consoleLayout.addWidget(consoleStatusButton, 2, 1, 1, 1);
    consoleLayout.addWidget(consoleClearButton, 2, 2, 1, 1);
    consoleLayout.addWidget(consoleSafety, 3, 0, 1, 4);
    consolePanel.setLayout(consoleLayout);

    let destinationLabel = form.create_label("Destination:");
    telec2Destination = form.create_combo();
    let fileLabel = form.create_label("File:");
    telec2File = form.create_textline();
    telec2File.setReadOnly(true);
    telec2File.setPlaceholder("Choose a local file to transfer...");
    let browseButton = form.create_button("Browse...");
    let uploadButton = form.create_button("Upload");
    let transferSafety = form.create_label("Transfer only: the selected file is never executed automatically.");
    let transferPanel = form.create_panel();
    let transferLayout = form.create_gridlayout();
    transferLayout.addWidget(destinationLabel, 0, 0, 1, 1);
    transferLayout.addWidget(telec2Destination, 0, 1, 1, 3);
    transferLayout.addWidget(fileLabel, 1, 0, 1, 1);
    transferLayout.addWidget(telec2File, 1, 1, 1, 2);
    transferLayout.addWidget(browseButton, 1, 3, 1, 1);
    transferLayout.addWidget(uploadButton, 2, 3, 1, 1);
    transferLayout.addWidget(transferSafety, 3, 0, 1, 4);
    transferPanel.setLayout(transferLayout);

    telec2Health = form.create_textmulti();
    telec2Health.setReadOnly(true);
    telec2Health.setText(
        "Select a connection to inspect it.\n\n" +
        "Bot creation, Bot-to-Bot Communication Mode, and Group Privacy Mode remain one-time @BotFather administration. " +
        "The operator token stays in the server-side mode-0600 configuration and is never returned to this client."
    );
    let healthPanel = form.create_panel();
    let healthLayout = form.create_gridlayout();
    healthLayout.addWidget(telec2Health, 0, 0, 1, 3);
    let healthVerifyButton = form.create_button("Run Health Check");
    healthLayout.addWidget(healthVerifyButton, 1, 0, 1, 1);
    healthPanel.setLayout(healthLayout);

    let tabs = form.create_tabs();
    tabs.addTab(consolePanel, "Console");
    tabs.addTab(transferPanel, "Transfer");
    tabs.addTab(healthPanel, "Health & Setup");

    let mainLayout = form.create_gridlayout();
    mainLayout.addWidget(connectionPanel, 0, 0, 1, 1);
    mainLayout.addWidget(tabs, 1, 0, 1, 1);
    telec2Dock = form.create_ext_dock("telec2-connections", "TeleC2", "bottom");
    telec2Dock.setLayout(mainLayout);
    telec2Dock.setSize(980, 680);

    form.connect(telec2Connections, "cellClicked", function(row, column) {
        telec2_select(telec2Connections.text(row, 0));
    });
    form.connect(telec2Connections, "cellDoubleClicked", function(row, column) {
        telec2_select(telec2Connections.text(row, 0));
        telec2_verify();
    });
    form.connect(telec2Destination, "currentTextChanged", function(name) {
        telec2_select(name);
    });
    form.connect(reloadButton, "clicked", function() {
        ax.service_command("TeleC2", "reload", {});
    });
    form.connect(verifyButton, "clicked", telec2_verify);
    form.connect(consoleVerifyButton, "clicked", telec2_verify);
    form.connect(healthVerifyButton, "clicked", telec2_verify);
    form.connect(statusButton, "clicked", telec2_status);
    form.connect(consoleStatusButton, "clicked", telec2_status);
    form.connect(consoleClearButton, "clicked", function() {
        if (telec2Active) {
            telec2Transcripts[telec2Active] = [];
            telec2Console.setText("");
        }
    });
    form.connect(browseButton, "clicked", function() {
        let selected = ax.prompt_open_file("Choose a file for TeleC2", "All files (*.*)");
        if (selected.length > 0) {
            telec2File.setText(selected);
            telec2_append(telec2Active, "Selected " + ax.file_basename(selected) + " (" + ax.format_size(ax.file_size(selected)) + ")");
        }
    });
    form.connect(uploadButton, "clicked", function() {
        let destination = telec2Destination.currentText();
        let path = telec2File.text();
        if (!destination || path.length === 0 || !ax.file_exists(path)) {
            ax.show_message("TeleC2", "Choose a configured connection and an existing local file.");
            return;
        }
        let size = ax.file_size(path);
        if (size <= 0 || size > telec2MaxBytes) {
            ax.show_message("TeleC2", "The file must be non-empty and no larger than " + ax.format_size(telec2MaxBytes) + ".");
            return;
        }
        let info = telec2Info[destination];
        let summary = "Connection: " + destination + " (" + info.platform + ")\n" +
                      "Bot: @" + info.bot_username + "\n" +
                      "File: " + ax.file_basename(path) + "\n" +
                      "Size: " + ax.format_size(size) + "\n\n" +
                      "Transfer this file? It will not be executed automatically.";
        if (!ax.prompt_confirm("Confirm TeleC2 transfer", summary)) {
            return;
        }
        telec2_append(destination, "Reading " + ax.file_basename(path) + "...");
        let content = ax.encode_file("base64", path);
        if (content.length === 0) {
            ax.show_message("TeleC2", "The selected file could not be read.");
            return;
        }
        ax.service_command("TeleC2", "upload", {
            destination: destination,
            filename: ax.file_basename(path),
            content_base64: content
        });
        telec2_append(destination, "Sending " + ax.file_basename(path) + "...");
    });

    ax.service_command("TeleC2", "list", {});
}

function telec2_update_connection(event) {
    let info = telec2Info[event.destination];
    if (!info) {
        return;
    }
    info.state = event.state || info.state;
    info.last_seen = event.last_seen || info.last_seen;
    info.last_message = event.last_message || info.last_message;
    let row = telec2_find_table_row(event.destination);
    if (row >= 0) {
        telec2Connections.setText(row, 3, info.state);
        telec2Connections.setText(row, 4, telec2_seen(info.last_seen));
    }
    if (event.last_message && event.state !== "Online") {
        telec2_append(event.destination, info.state + ": " + event.last_message);
    }
    if (event.destination === telec2Active) {
        telec2_select(event.destination);
    }
}

function data_handler(data) {
    let event = null;
    try {
        event = JSON.parse(data);
    } catch (error) {
        if (telec2Active) {
            telec2_append(telec2Active, "Invalid service response: " + error);
        }
        return;
    }

    if (event.type === "config") {
        telec2Connections.clear();
        telec2Destination.clear();
        telec2Items = [];
        telec2Info = {};
        if (!event.ok) {
            telec2Health.setText("Configuration error:\n" + event.message);
            return;
        }
        telec2MaxBytes = event.max_file_size_bytes;
        for (let item of event.destinations) {
            item.state = item.state || "Configured";
            telec2Items.push(item);
            telec2Info[item.name] = item;
            telec2Connections.addItem([
                item.name,
                item.platform.toUpperCase(),
                "@" + item.bot_username,
                item.state,
                telec2_seen(item.last_seen)
            ]);
            telec2Destination.addItem(item.name);
            if (!telec2Transcripts[item.name]) {
                telec2Transcripts[item.name] = [];
            }
        }
        telec2Connections.setReadOnly(true);
        if (telec2Items.length > 0) {
            let selected = telec2Info[telec2Active] ? telec2Active : telec2Items[0].name;
            telec2_select(selected);
            telec2_append(selected, "Loaded " + telec2Items.length + " configured connection(s).");
        }
        return;
    }

    if (event.type === "connection") {
        telec2_update_connection(event);
        return;
    }

    let destination = event.destination || telec2Active;
    if (event.type === "telec2_reply") {
        telec2_append(destination, "TeleC2: " + event.message);
        return;
    }

    let prefix = event.ok ? "OK" : "ERROR";
    telec2_append(destination, prefix + ": " + event.message);
    if (event.operator_bot_username) {
        telec2_append(destination, "Operator bot: @" + event.operator_bot_username + " (" + event.operator_bot_id + ")");
    }
    if (event.sha256) {
        telec2_append(destination, "SHA-256: " + event.sha256);
    }
}
