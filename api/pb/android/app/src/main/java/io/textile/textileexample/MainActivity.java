package io.textile.textileexample;

import android.os.Bundle;
import android.support.v7.app.AppCompatActivity;
import android.view.View;

import io.textile.threads.Client;
import io.textile.threads.Config;
import io.textile.threads.DefaultConfig;
import io.textile.threads.TextileConfig;

public class MainActivity extends AppCompatActivity {

    Client client;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        System.out.println("Info: " + "Startup");
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);
        initIPFS();
    }

    public void onButtonClick(View v) {
        try {
            String storeId = client.NewStoreSync();
            System.out.println("Thread Info: " + storeId);
        } catch (Exception e) {
            System.out.println("Thread Info: " + e.getMessage());
        }
    }

    private void initIPFS() {
        try {
            /**
             * To use hosted threads from Textile, use the following instead of DefaultConfig
             */
            /*
                Config config = new TextileConfig(
                    PROJECT_TOKEN,
                    DEVICE_UUID
                );
                // Optionally, restore a session for the same user by supplying the session ID
                config.setSession(SESSION_ID);
                ...
                // If it's your first time starting the client, you can get and locally store the session id
                String SESSION_ID = client.getSession();
                // The above only works when using the TextileConfig
            */

            Config config = new DefaultConfig();
            client = new Client(config);
            client.init((success)->{
                System.out.println("Thread Info: " + "READY!");
                // System.out.println("Session Info: " + client.getSession());
            });
        } catch (Exception e) {
            System.out.println("Thread Info: " + e.getMessage());
        }
    }
}
