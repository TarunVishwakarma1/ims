package com.tv.ims

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.material3.adaptive.navigationsuite.NavigationSuiteScaffold
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.tooling.preview.PreviewScreenSizes
import androidx.compose.ui.unit.dp
import com.tv.ims.ui.theme.ImsTheme

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            ImsTheme {
                ImsApp()
            }
        }
    }
}

@PreviewScreenSizes
@Composable
fun ImsApp() {
    var currentDestination by rememberSaveable { mutableStateOf(AppDestinations.HOME) }

    NavigationSuiteScaffold(
        navigationSuiteItems = {
            AppDestinations.entries.forEach {
                item(
                    icon = {
                        Icon(
                            painterResource(it.icon),
                            contentDescription = it.label,
                        )
                    },
                    label = { Text(it.label) },
                    selected = it == currentDestination,
                    onClick = { currentDestination = it }
                )
            }
        }
    ) {
        Scaffold(modifier = Modifier.fillMaxSize()) { innerPadding ->
            Box(modifier = Modifier.padding(innerPadding)) {
                when (currentDestination) {
                    AppDestinations.HOME -> HomeContent()
                    AppDestinations.FAVORITES -> FavoritesContent()
                    AppDestinations.PROFILE -> ProfileContent()
                }
            }
        }
    }
}

enum class AppDestinations(
    val label: String,
    val icon: Int,
) {
    HOME("Home", R.drawable.ic_home),
    FAVORITES("Favorites", R.drawable.ic_favorite),
    PROFILE("Profile", R.drawable.ic_account_box),
}

@Composable
fun HomeContent(modifier: Modifier = Modifier) {
    LazyColumn(
        modifier = modifier
            .fillMaxSize()
            .padding(horizontal = 16.dp),
        verticalArrangement = Arrangement.spacedBy(20.dp)
    ) {
        item {
            Text(
                text = "Home Content",
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(top = 24.dp)
            )
        }
        item {
            Column {
                Text(
                    text = "Welcome Back!",
                    style = MaterialTheme.typography.headlineLarge,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = "What are you looking for today?",
                    style = MaterialTheme.typography.bodyLarge,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
        item {
            Card(
                shape = RoundedCornerShape(16.dp),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant),
                elevation = CardDefaults.cardElevation(defaultElevation = 2.dp),
                modifier = Modifier
                    .fillMaxWidth()
                    .height(150.dp)
            ) {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center
                ) {
                    Text(
                        text = "Promotional Banner",
                        style = MaterialTheme.typography.titleMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

@Composable
fun FavoritesContent(modifier: Modifier = Modifier) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .padding(16.dp),
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Text(
            text = "Favorites Content",
            style = MaterialTheme.typography.titleMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(top = 24.dp, bottom = 40.dp)
        )
        
        Icon(
            imageVector = Icons.Default.FavoriteBorder,
            contentDescription = null,
            modifier = Modifier.size(60.dp),
            tint = MaterialTheme.colorScheme.onSurfaceVariant
        )
        
        Spacer(modifier = Modifier.height(16.dp))
        
        Text(
            text = "No favorites yet",
            style = MaterialTheme.typography.titleLarge,
            fontWeight = FontWeight.SemiBold
        )
        Text(
            text = "Save items you love to find them here later.",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center,
            modifier = Modifier.padding(top = 4.dp, start = 24.dp, end = 24.dp)
        )
    }
}

@Composable
fun ProfileContent(modifier: Modifier = Modifier) {
    LazyColumn(
        modifier = modifier.fillMaxSize()
    ) {
        item {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp, vertical = 24.dp),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Surface(
                    shape = CircleShape,
                    color = MaterialTheme.colorScheme.primaryContainer,
                    modifier = Modifier.size(60.dp)
                ) {
                    Icon(
                        imageVector = Icons.Default.Person,
                        contentDescription = null,
                        tint = MaterialTheme.colorScheme.onPrimaryContainer,
                        modifier = Modifier
                            .padding(12.dp)
                            .fillMaxSize()
                    )
                }
                
                Column(modifier = Modifier.padding(start = 16.dp)) {
                    Text(
                        text = "Profile Content",
                        style = MaterialTheme.typography.labelMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        text = "Tarun Vishwakarma",
                        style = MaterialTheme.typography.titleLarge,
                        fontWeight = FontWeight.SemiBold
                    )
                    Text(
                        text = "tarun@example.com",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
        item { HorizontalDivider(color = MaterialTheme.colorScheme.surfaceVariant) }
        
        item { ProfileMenuItem(Icons.Default.ShoppingCart, "Orders") }
        item { ProfileMenuItem(Icons.Default.CreditCard, "Payment Methods") }
        item { ProfileMenuItem(Icons.Default.LocationOn, "Addresses") }
        
        item { HorizontalDivider(color = MaterialTheme.colorScheme.surfaceVariant) }
        
        item { ProfileMenuItem(Icons.Default.Settings, "Settings") }
        item { ProfileMenuItem(Icons.Default.Info, "Help & Support") }
        
        item { HorizontalDivider(color = MaterialTheme.colorScheme.surfaceVariant) }
        
        item {
            Text(
                text = "Log Out",
                color = MaterialTheme.colorScheme.error,
                modifier = Modifier
                    .fillMaxWidth()
                    .clickable { /* Handle Log out */ }
                    .padding(16.dp)
            )
        }
    }
}

@Composable
fun ProfileMenuItem(icon: androidx.compose.ui.graphics.vector.ImageVector, title: String) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable { }
            .padding(16.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Icon(
            imageVector = icon,
            contentDescription = null,
            tint = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Spacer(modifier = Modifier.width(16.dp))
        Text(
            text = title,
            style = MaterialTheme.typography.bodyLarge
        )
    }
}

@Preview(showBackground = true)
@Composable
fun GreetingPreview() {
    ImsTheme {
        HomeContent()
    }
}