<?php
// PHP FastCGI Demo Application
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>PHP FastCGI Demo</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .info-box { background: #f0f0f0; padding: 20px; margin: 20px 0; border-radius: 5px; }
        .params { background: #e8f4fd; padding: 15px; margin: 10px 0; border-radius: 3px; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
    </style>
</head>
<body>
    <h1>🐘 PHP FastCGI Demo Application</h1>
    
    <div class="info-box">
        <h2>Application Info</h2>
        <p><strong>PHP Version:</strong> <?= PHP_VERSION ?></p>
        <p><strong>Server API:</strong> <?= PHP_SAPI ?></p>
        <p><strong>Current Time:</strong> <?= date('Y-m-d H:i:s') ?></p>
        <p><strong>Document Root:</strong> <?= $_SERVER['DOCUMENT_ROOT'] ?? 'Not set' ?></p>
        <p><strong>Script Name:</strong> <?= $_SERVER['SCRIPT_NAME'] ?? 'Not set' ?></p>
    </div>

    <div class="info-box">
        <h2>FastCGI Parameters</h2>
        <div class="params">
            <table>
                <tr><th>Parameter</th><th>Value</th></tr>
                <?php
                $fastcgi_params = [
                    'SCRIPT_FILENAME', 'QUERY_STRING', 'REQUEST_METHOD', 'CONTENT_TYPE', 
                    'CONTENT_LENGTH', 'SCRIPT_NAME', 'REQUEST_URI', 'DOCUMENT_URI', 
                    'DOCUMENT_ROOT', 'SERVER_PROTOCOL', 'REQUEST_SCHEME', 'HTTPS',
                    'GATEWAY_INTERFACE', 'SERVER_SOFTWARE', 'REMOTE_ADDR', 'REMOTE_PORT',
                    'SERVER_ADDR', 'SERVER_PORT', 'SERVER_NAME', 'REDIRECT_STATUS',
                    'PATH_INFO', 'PATH_TRANSLATED'
                ];
                
                foreach ($fastcgi_params as $param) {
                    $value = $_SERVER[$param] ?? '<em>Not set</em>';
                    echo "<tr><td><strong>$param</strong></td><td>$value</td></tr>";
                }
                ?>
            </table>
        </div>
    </div>

    <div class="info-box">
        <h2>Custom Environment Variables</h2>
        <div class="params">
            <table>
                <tr><th>Variable</th><th>Value</th></tr>
                <?php
                $custom_vars = ['APP_ENV', 'APP_DEBUG', 'DB_HOST', 'DB_NAME'];
                foreach ($custom_vars as $var) {
                    $value = $_SERVER[$var] ?? '<em>Not set</em>';
                    echo "<tr><td><strong>$var</strong></td><td>$value</td></tr>";
                }
                ?>
            </table>
        </div>
    </div>

    <div class="info-box">
        <h2>Request Headers</h2>
        <div class="params">
            <table>
                <tr><th>Header</th><th>Value</th></tr>
                <?php
                foreach (getallheaders() as $name => $value) {
                    echo "<tr><td><strong>$name</strong></td><td>$value</td></tr>";
                }
                ?>
            </table>
        </div>
    </div>
    
    <div class="info-box">
        <h2>Test Database Connection</h2>
        <?php
        $db_host = $_SERVER['DB_HOST'] ?? 'db';
        $db_name = $_SERVER['DB_NAME'] ?? 'myapp';
        $db_user = 'appuser';
        $db_pass = 'apppass';
        
        try {
            $pdo = new PDO("mysql:host=$db_host;dbname=$db_name", $db_user, $db_pass);
            echo "<p style='color: green;'>✅ Database connection successful!</p>";
            echo "<p>Connected to: $db_host/$db_name</p>";
        } catch (PDOException $e) {
            echo "<p style='color: red;'>❌ Database connection failed: " . $e->getMessage() . "</p>";
        }
        ?>
    </div>

    <div class="info-box">
        <h2>PHP Info Link</h2>
        <p><a href="info.php" target="_blank">View Full PHP Info</a></p>
    </div>
</body>
</html>