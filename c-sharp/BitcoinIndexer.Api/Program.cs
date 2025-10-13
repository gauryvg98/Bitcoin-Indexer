using BitcoinIndexer.Application.Services;
using BitcoinIndexer.Core.Interfaces;
using BitcoinIndexer.Infrastructure.Extensions;

var builder = WebApplication.CreateBuilder(args);

// Add services to the container
builder.Services.AddControllers();
builder.Services.AddEndpointsApiExplorer();
builder.Services.AddSwaggerGen();

// Add configuration
builder.Services.AddIndexerConfiguration(builder.Configuration);

// Add database
builder.Services.AddIndexerDatabase(builder.Configuration);

// Add Bitcoin RPC
builder.Services.AddBitcoinRpc();

// Add Bitcoin indexer
builder.Services.AddScoped<IBitcoinIndexerService, BitcoinIndexerService>();
builder.Services.AddHostedService<BitcoinIndexerHostedService>();

var app = builder.Build();

// Configure the HTTP request pipeline
if (app.Environment.IsDevelopment())
{
    app.UseSwagger();
    app.UseSwaggerUI();
}

app.UseHttpsRedirection();
app.UseAuthorization();
app.MapControllers();

app.Run();
