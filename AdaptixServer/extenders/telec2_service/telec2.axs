var telec2Dock = null;
var telec2Destination = null;
var telec2File = null;
var telec2Log = null;
var telec2MaxBytes = 20 * 1024 * 1024;
var telec2Platforms = {};

function telec2_log(message) {
    if (telec2Log !== null) {
        telec2Log.appendText("[" + new Date().toLocaleTimeString() + "] " + message);
    }
}

function InitService() {
    let destinationLabel = form.create_label("Destination:");
    telec2Destination = form.create_combo();

    let fileLabel = form.create_label("File:");
    telec2File = form.create_textline();
    telec2File.setReadOnly(true);
    telec2File.setPlaceholder("Choose a local file to transfer...");
    let browseButton = form.create_button("Browse...");

    let refreshButton = form.create_button("Refresh");
    let statusButton = form.create_button("Status");
    let uploadButton = form.create_button("Upload");

    let safetyLabel = form.create_label("Transfer only: this panel never executes uploaded files.");
    telec2Log = form.create_textmulti();
    telec2Log.setReadOnly(true);
    telec2Log.setPlaceholder("TeleC2 service activity appears here...");

    let controls = form.create_gridlayout();
    controls.addWidget(destinationLabel, 0, 0, 1, 1);
    controls.addWidget(telec2Destination, 0, 1, 1, 3);
    controls.addWidget(fileLabel, 1, 0, 1, 1);
    controls.addWidget(telec2File, 1, 1, 1, 2);
    controls.addWidget(browseButton, 1, 3, 1, 1);
    controls.addWidget(refreshButton, 2, 0, 1, 1);
    controls.addWidget(statusButton, 2, 1, 1, 1);
    controls.addWidget(uploadButton, 2, 3, 1, 1);
    controls.addWidget(safetyLabel, 3, 0, 1, 4);
    controls.addWidget(telec2Log, 4, 0, 1, 4);

    telec2Dock = form.create_ext_dock("telec2-transfer", "TeleC2", "bottom");
    telec2Dock.setLayout(controls);
    telec2Dock.setSize(760, 420);

    form.connect(browseButton, "clicked", function() {
        let selected = ax.prompt_open_file("Choose a file for TeleC2", "All files (*.*)");
        if (selected.length > 0) {
            telec2File.setText(selected);
            telec2_log("Selected " + ax.file_basename(selected) + " (" + ax.format_size(ax.file_size(selected)) + ")");
        }
    });

    form.connect(refreshButton, "clicked", function() {
        ax.service_command("TeleC2", "reload", {});
    });

    form.connect(statusButton, "clicked", function() {
        let destination = telec2Destination.currentText();
        if (destination.length === 0) {
            ax.show_message("TeleC2", "Configure at least one TeleC2 destination first.");
            return;
        }
        ax.service_command("TeleC2", "status", {destination: destination});
        telec2_log("Requesting status from " + destination + "...");
    });

    form.connect(uploadButton, "clicked", function() {
        let destination = telec2Destination.currentText();
        let path = telec2File.text();
        if (destination.length === 0 || path.length === 0 || !ax.file_exists(path)) {
            ax.show_message("TeleC2", "Choose a configured destination and an existing local file.");
            return;
        }

        let size = ax.file_size(path);
        if (size <= 0 || size > telec2MaxBytes) {
            ax.show_message("TeleC2", "The file must be non-empty and no larger than " + ax.format_size(telec2MaxBytes) + ".");
            return;
        }

        let platform = telec2Platforms[destination] || "unknown";
        let summary = "Destination: " + destination + " (" + platform + ")\n" +
                      "File: " + ax.file_basename(path) + "\n" +
                      "Size: " + ax.format_size(size) + "\n\n" +
                      "Transfer this file? It will not be executed automatically.";
        if (!ax.prompt_confirm("Confirm TeleC2 transfer", summary)) {
            return;
        }

        telec2_log("Reading " + ax.file_basename(path) + "...");
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
        telec2_log("Sending " + ax.file_basename(path) + " to " + destination + "...");
    });

    ax.service_command("TeleC2", "list", {});
}

function data_handler(data) {
    let event = null;
    try {
        event = JSON.parse(data);
    } catch (error) {
        telec2_log("Invalid service response: " + error);
        return;
    }

    if (event.type === "config") {
        telec2Destination.clear();
        telec2Platforms = {};
        if (!event.ok) {
            telec2_log("Configuration: " + event.message);
            return;
        }
        telec2MaxBytes = event.max_file_size_bytes;
        for (let destination of event.destinations) {
            telec2Destination.addItem(destination.name);
            telec2Platforms[destination.name] = destination.platform;
        }
        telec2_log("Loaded " + event.destinations.length + " destination(s).");
        return;
    }

    let prefix = event.ok ? "OK" : "ERROR";
    let destination = event.destination ? " [" + event.destination + "]" : "";
    telec2_log(prefix + destination + ": " + event.message);
    if (event.sha256) {
        telec2_log("SHA-256: " + event.sha256);
    }
}
