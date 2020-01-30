package io.textile.textileexample;

import android.os.Bundle;
import android.support.v7.app.AppCompatActivity;
import android.view.View;

import io.textile.threads.Client;

public class MainActivity extends AppCompatActivity {

    Client client;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        initIPFS();
    }

    public void onButtonClick(View v) {
        try {
            String storeId = client.NewStoreSync();
            System.out.println("Success: " + storeId);
        } catch (Exception e) {
            System.out.println(e.getMessage());
        }
    }

    private void initIPFS() {
        try {
            client = new Client("localhost", 6006);
            client.Connect();
        } catch (Exception e) {
            System.out.println(e.getMessage());
        }
    }
}
