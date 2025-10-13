using System;
using Microsoft.EntityFrameworkCore.Migrations;
using Npgsql.EntityFrameworkCore.PostgreSQL.Metadata;

#nullable disable

namespace BitcoinIndexer.Infrastructure.Migrations
{
    /// <inheritdoc />
    public partial class InitialCreate : Migration
    {
        /// <inheritdoc />
        protected override void Up(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.CreateTable(
                name: "address_metadata",
                columns: table => new
                {
                    address = table.Column<string>(type: "character varying(100)", maxLength: 100, nullable: false),
                    first_seen_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: true),
                    last_seen_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: true),
                    total_received_sats = table.Column<long>(type: "bigint", nullable: false),
                    total_sent_sats = table.Column<long>(type: "bigint", nullable: false),
                    transaction_count = table.Column<int>(type: "integer", nullable: false),
                    is_watched = table.Column<bool>(type: "boolean", nullable: false),
                    created_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: false),
                    updated_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: true)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_address_metadata", x => x.address);
                });

            migrationBuilder.CreateTable(
                name: "block_info",
                columns: table => new
                {
                    hash = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: false),
                    height = table.Column<int>(type: "integer", nullable: false),
                    previous_hash = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: true),
                    timestamp = table.Column<DateTime>(type: "timestamp with time zone", nullable: false),
                    size = table.Column<int>(type: "integer", nullable: false),
                    weight = table.Column<int>(type: "integer", nullable: false),
                    transaction_count = table.Column<int>(type: "integer", nullable: false),
                    version = table.Column<int>(type: "integer", nullable: false),
                    nonce = table.Column<long>(type: "bigint", nullable: false),
                    difficulty_target = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: true),
                    merkle_root = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: true),
                    created_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: false),
                    updated_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: true)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_block_info", x => x.hash);
                });

            migrationBuilder.CreateTable(
                name: "index_progress",
                columns: table => new
                {
                    id = table.Column<bool>(type: "boolean", nullable: false),
                    last_height = table.Column<int>(type: "integer", nullable: false),
                    last_block_hash = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: false),
                    updated_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_index_progress", x => x.id);
                });

            migrationBuilder.CreateTable(
                name: "transaction",
                columns: table => new
                {
                    txid = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: false),
                    block_height = table.Column<int>(type: "integer", nullable: true),
                    block_hash = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: true),
                    block_time = table.Column<DateTime>(type: "timestamp with time zone", nullable: true),
                    size = table.Column<int>(type: "integer", nullable: true),
                    weight = table.Column<int>(type: "integer", nullable: true),
                    fee_sats = table.Column<long>(type: "bigint", nullable: true),
                    is_coinbase = table.Column<bool>(type: "boolean", nullable: false),
                    created_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: false),
                    updated_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: true)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_transaction", x => x.txid);
                });

            migrationBuilder.CreateTable(
                name: "tx_reference",
                columns: table => new
                {
                    id = table.Column<int>(type: "integer", nullable: false)
                        .Annotation("Npgsql:ValueGenerationStrategy", NpgsqlValueGenerationStrategy.IdentityByDefaultColumn),
                    txid = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: false),
                    address = table.Column<string>(type: "character varying(100)", maxLength: 100, nullable: false),
                    direction = table.Column<string>(type: "character varying(10)", maxLength: 10, nullable: false),
                    value_sats = table.Column<long>(type: "bigint", nullable: false),
                    sender_address = table.Column<string>(type: "character varying(100)", maxLength: 100, nullable: true),
                    receiver_address = table.Column<string>(type: "character varying(100)", maxLength: 100, nullable: true),
                    block_height = table.Column<int>(type: "integer", nullable: true),
                    block_timestamp = table.Column<int>(type: "integer", nullable: true),
                    created_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_tx_reference", x => x.id);
                });

            migrationBuilder.CreateTable(
                name: "utxo",
                columns: table => new
                {
                    txid = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: false),
                    vout = table.Column<int>(type: "integer", nullable: false),
                    address = table.Column<string>(type: "character varying(100)", maxLength: 100, nullable: false),
                    script_hex = table.Column<string>(type: "character varying(10000)", maxLength: 10000, nullable: false),
                    value_sats = table.Column<long>(type: "bigint", nullable: false),
                    status = table.Column<string>(type: "character varying(50)", maxLength: 50, nullable: false),
                    block_height = table.Column<int>(type: "integer", nullable: true),
                    block_hash = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: true),
                    first_seen_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: false),
                    updated_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: true),
                    spent_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: true),
                    spent_by_txid = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: true)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_utxo", x => new { x.txid, x.vout });
                });

            migrationBuilder.CreateTable(
                name: "watch_target",
                columns: table => new
                {
                    id = table.Column<long>(type: "bigint", nullable: false)
                        .Annotation("Npgsql:ValueGenerationStrategy", NpgsqlValueGenerationStrategy.IdentityByDefaultColumn),
                    kind = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: false),
                    address = table.Column<string>(type: "character varying(100)", maxLength: 100, nullable: true),
                    xpub = table.Column<string>(type: "character varying(200)", maxLength: 200, nullable: true),
                    derivation_scheme = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: true),
                    account = table.Column<int>(type: "integer", nullable: true),
                    gap_limit = table.Column<int>(type: "integer", nullable: false),
                    created_at = table.Column<DateTime>(type: "timestamp with time zone", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_watch_target", x => x.id);
                });

            migrationBuilder.CreateTable(
                name: "transaction_input",
                columns: table => new
                {
                    txid = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: false),
                    vout = table.Column<int>(type: "integer", nullable: false),
                    prev_txid = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: true),
                    prev_vout = table.Column<int>(type: "integer", nullable: false),
                    script_sig = table.Column<string>(type: "character varying(10000)", maxLength: 10000, nullable: true),
                    sequence = table.Column<long>(type: "bigint", nullable: false),
                    witness = table.Column<string>(type: "character varying(10000)", maxLength: 10000, nullable: true),
                    transaction_txid = table.Column<string>(type: "character varying(64)", nullable: true)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_transaction_input", x => new { x.txid, x.vout });
                    table.ForeignKey(
                        name: "FK_transaction_input_transaction_transaction_txid",
                        column: x => x.transaction_txid,
                        principalTable: "transaction",
                        principalColumn: "txid");
                });

            migrationBuilder.CreateTable(
                name: "transaction_output",
                columns: table => new
                {
                    txid = table.Column<string>(type: "character varying(64)", maxLength: 64, nullable: false),
                    vout = table.Column<int>(type: "integer", nullable: false),
                    value_sats = table.Column<long>(type: "bigint", nullable: false),
                    address = table.Column<string>(type: "character varying(100)", maxLength: 100, nullable: false),
                    script_type = table.Column<string>(type: "character varying(50)", maxLength: 50, nullable: true),
                    script_hex = table.Column<string>(type: "character varying(10000)", maxLength: 10000, nullable: true),
                    script_asm = table.Column<string>(type: "character varying(10000)", maxLength: 10000, nullable: true),
                    transaction_txid = table.Column<string>(type: "character varying(64)", nullable: true)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_transaction_output", x => new { x.txid, x.vout });
                    table.ForeignKey(
                        name: "FK_transaction_output_transaction_transaction_txid",
                        column: x => x.transaction_txid,
                        principalTable: "transaction",
                        principalColumn: "txid");
                });

            migrationBuilder.CreateTable(
                name: "watched_script",
                columns: table => new
                {
                    id = table.Column<long>(type: "bigint", nullable: false)
                        .Annotation("Npgsql:ValueGenerationStrategy", NpgsqlValueGenerationStrategy.IdentityByDefaultColumn),
                    address = table.Column<string>(type: "character varying(100)", maxLength: 100, nullable: false),
                    script_hex = table.Column<string>(type: "character varying(10000)", maxLength: 10000, nullable: false),
                    type = table.Column<string>(type: "character varying(20)", maxLength: 20, nullable: true),
                    target_id = table.Column<long>(type: "bigint", nullable: false)
                },
                constraints: table =>
                {
                    table.PrimaryKey("PK_watched_script", x => x.id);
                    table.ForeignKey(
                        name: "FK_watched_script_watch_target_target_id",
                        column: x => x.target_id,
                        principalTable: "watch_target",
                        principalColumn: "id",
                        onDelete: ReferentialAction.Cascade);
                });

            migrationBuilder.CreateIndex(
                name: "IX_address_metadata_is_watched",
                table: "address_metadata",
                column: "is_watched");

            migrationBuilder.CreateIndex(
                name: "IX_address_metadata_last_seen_at",
                table: "address_metadata",
                column: "last_seen_at");

            migrationBuilder.CreateIndex(
                name: "IX_block_info_height",
                table: "block_info",
                column: "height");

            migrationBuilder.CreateIndex(
                name: "IX_block_info_timestamp",
                table: "block_info",
                column: "timestamp");

            migrationBuilder.CreateIndex(
                name: "IX_transaction_block_hash",
                table: "transaction",
                column: "block_hash");

            migrationBuilder.CreateIndex(
                name: "IX_transaction_block_height",
                table: "transaction",
                column: "block_height");

            migrationBuilder.CreateIndex(
                name: "IX_transaction_block_time",
                table: "transaction",
                column: "block_time");

            migrationBuilder.CreateIndex(
                name: "IX_transaction_is_coinbase",
                table: "transaction",
                column: "is_coinbase");

            migrationBuilder.CreateIndex(
                name: "IX_transaction_input_prev_txid_prev_vout",
                table: "transaction_input",
                columns: new[] { "prev_txid", "prev_vout" });

            migrationBuilder.CreateIndex(
                name: "IX_transaction_input_transaction_txid",
                table: "transaction_input",
                column: "transaction_txid");

            migrationBuilder.CreateIndex(
                name: "IX_transaction_input_txid",
                table: "transaction_input",
                column: "txid");

            migrationBuilder.CreateIndex(
                name: "IX_transaction_output_address",
                table: "transaction_output",
                column: "address");

            migrationBuilder.CreateIndex(
                name: "IX_transaction_output_transaction_txid",
                table: "transaction_output",
                column: "transaction_txid");

            migrationBuilder.CreateIndex(
                name: "IX_transaction_output_txid",
                table: "transaction_output",
                column: "txid");

            migrationBuilder.CreateIndex(
                name: "IX_tx_reference_address",
                table: "tx_reference",
                column: "address");

            migrationBuilder.CreateIndex(
                name: "IX_tx_reference_block_height",
                table: "tx_reference",
                column: "block_height");

            migrationBuilder.CreateIndex(
                name: "IX_tx_reference_created_at",
                table: "tx_reference",
                column: "created_at");

            migrationBuilder.CreateIndex(
                name: "IX_tx_reference_txid",
                table: "tx_reference",
                column: "txid");

            migrationBuilder.CreateIndex(
                name: "IX_tx_reference_txid_address_direction",
                table: "tx_reference",
                columns: new[] { "txid", "address", "direction" },
                unique: true);

            migrationBuilder.CreateIndex(
                name: "IX_utxo_address",
                table: "utxo",
                column: "address");

            migrationBuilder.CreateIndex(
                name: "IX_utxo_address_status_block_height",
                table: "utxo",
                columns: new[] { "address", "status", "block_height" });

            migrationBuilder.CreateIndex(
                name: "IX_utxo_block_hash",
                table: "utxo",
                column: "block_hash");

            migrationBuilder.CreateIndex(
                name: "IX_utxo_block_height",
                table: "utxo",
                column: "block_height");

            migrationBuilder.CreateIndex(
                name: "IX_watch_target_address",
                table: "watch_target",
                column: "address",
                unique: true,
                filter: "\"address\" IS NOT NULL");

            migrationBuilder.CreateIndex(
                name: "IX_watched_script_address",
                table: "watched_script",
                column: "address");

            migrationBuilder.CreateIndex(
                name: "IX_watched_script_script_hex",
                table: "watched_script",
                column: "script_hex",
                unique: true);

            migrationBuilder.CreateIndex(
                name: "IX_watched_script_target_id",
                table: "watched_script",
                column: "target_id");

            migrationBuilder.CreateIndex(
                name: "IX_watched_script_type",
                table: "watched_script",
                column: "type");
        }

        /// <inheritdoc />
        protected override void Down(MigrationBuilder migrationBuilder)
        {
            migrationBuilder.DropTable(
                name: "address_metadata");

            migrationBuilder.DropTable(
                name: "block_info");

            migrationBuilder.DropTable(
                name: "index_progress");

            migrationBuilder.DropTable(
                name: "transaction_input");

            migrationBuilder.DropTable(
                name: "transaction_output");

            migrationBuilder.DropTable(
                name: "tx_reference");

            migrationBuilder.DropTable(
                name: "utxo");

            migrationBuilder.DropTable(
                name: "watched_script");

            migrationBuilder.DropTable(
                name: "transaction");

            migrationBuilder.DropTable(
                name: "watch_target");
        }
    }
}
