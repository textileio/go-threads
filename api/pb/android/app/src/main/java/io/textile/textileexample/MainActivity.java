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
            Config config = new TextileConfig(
                    "54e24fc3-fda5-478a-b1f7-040ea5aaab33", "77b7b712-5ec4-42f2-b3c4-57b22f0cf057"
            );
//            Config config = new DefaultConfig();
            client = new Client(config);
            client.init((success)->{
                System.out.println("Thread Info: " + "READY!");
            });
        } catch (Exception e) {
            System.out.println("Thread Info: " + e.getMessage());
        }
    }
}
