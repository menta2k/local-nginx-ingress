<?php
// Simple PHP application
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Simple PHP FastCGI</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f9f9f9; }
        .container { background: white; padding: 30px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
    </style>
</head>
<body>
    <div class="container">
        <h1>🚀 Simple PHP FastCGI Demo</h1>
        
        <p><strong>This is a simple PHP application using FastCGI with label-based parameters.</strong></p>
        
        <h2>Basic Information</h2>
        <ul>
            <li><strong>PHP Version:</strong> <?= PHP_VERSION ?></li>
            <li><strong>Server API:</strong> <?= PHP_SAPI ?></li>
            <li><strong>Current Time:</strong> <?= date('Y-m-d H:i:s') ?></li>
            <li><strong>Request Method:</strong> <?= $_SERVER['REQUEST_METHOD'] ?? 'Unknown' ?></li>
            <li><strong>Request URI:</strong> <?= $_SERVER['REQUEST_URI'] ?? 'Unknown' ?></li>
        </ul>
        
        <h2>Key FastCGI Parameters</h2>
        <ul>
            <li><strong>SCRIPT_FILENAME:</strong> <?= $_SERVER['SCRIPT_FILENAME'] ?? 'Not set' ?></li>
            <li><strong>DOCUMENT_ROOT:</strong> <?= $_SERVER['DOCUMENT_ROOT'] ?? 'Not set' ?></li>
            <li><strong>HTTP_HOST:</strong> <?= $_SERVER['HTTP_HOST'] ?? 'Not set' ?></li>
            <li><strong>SERVER_NAME:</strong> <?= $_SERVER['SERVER_NAME'] ?? 'Not set' ?></li>
        </ul>
        
        <p><em>This application uses FastCGI parameters defined directly in Docker labels.</em></p>
        
        <h2>Test Form</h2>
        <form method="POST">
            <label for="message">Message:</label><br>
            <input type="text" name="message" id="message" value="<?= htmlspecialchars($_POST['message'] ?? '') ?>" style="width: 200px; padding: 5px;"><br><br>
            <button type="submit" style="padding: 8px 15px; background: #007cba; color: white; border: none; border-radius: 3px;">Submit</button>
        </form>
        
        <?php if (!empty($_POST['message'])): ?>
            <div style="background: #d4edda; color: #155724; padding: 10px; margin-top: 15px; border-radius: 5px;">
                <strong>You submitted:</strong> <?= htmlspecialchars($_POST['message']) ?>
            </div>
        <?php endif; ?>
    </div>
</body>
</html>