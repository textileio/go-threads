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
            Config config = new DefaultConfig();
            /**
             * Replace the above with your Textile cloud info to run with cloud.textile.io
             */
            //Config config = new TextileConfig(
            //        "PROJECT TOKEN, "DEVICE UUID"
            //);
            client = new Client(config);
            client.init((success)->{
                System.out.println("Thread Info: " + "READY!");
            });
        } catch (Exception e) {
            System.out.println("Thread Info: " + e.getMessage());
        }
    }
}
