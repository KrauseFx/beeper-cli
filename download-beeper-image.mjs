#!/usr/bin/env node
/**
 * Download and save a Beeper image using beepcli
 * Usage: node download-beeper-image.mjs <event_id> <output_path>
 */

import { copyFile } from 'fs/promises';
import sqlite3 from 'sqlite3';
import { exec } from 'child_process';
import { promisify } from 'util';
import path from 'path';
import { fileURLToPath } from 'url';

const execAsync = promisify(exec);
const __dirname = path.dirname(fileURLToPath(import.meta.url));

const DB_PATH = `${process.env.HOME}/Library/Application Support/BeeperTexts/index.db`;
const BEEPCLI_PATH = path.resolve(__dirname, '../beepcli');

async function getImageInfo(eventId) {
  return new Promise((resolve, reject) => {
    const db = new sqlite3.Database(DB_PATH, sqlite3.OPEN_READONLY);
    
    db.get(
      'SELECT message FROM mx_room_messages WHERE eventID = ?',
      [eventId],
      (err, row) => {
        db.close();
        if (err) return reject(err);
        if (!row) return reject(new Error(`Event not found: ${eventId}`));
        
        const msg = JSON.parse(row.message);
        if (!msg.attachments || msg.attachments.length === 0) {
          return reject(new Error('No attachments found'));
        }
        
        resolve(msg.attachments[0]);
      }
    );
  });
}

async function downloadImage(mxcUrl, outputPath) {
  // Use beepcli to get the local media path
  const cmd = `cd ${BEEPCLI_PATH} && node dist/cli.js download '${mxcUrl.replace(/'/g, "'\\''")}' `;
  
  try {
    const { stdout } = await execAsync(cmd);
    
    // Parse the output to get the local path
    const pathMatch = stdout.match(/Path: (.+)/);
    if (!pathMatch) {
      throw new Error('Could not parse beepcli output');
    }
    
    // Decode URL-encoded path
    const localPath = decodeURIComponent(pathMatch[1].trim());
    
    // Copy the file to the output path
    await copyFile(localPath, outputPath);
    console.log(`✅ Copied from local cache to: ${outputPath}`);
    
    return true;
  } catch (error) {
    throw new Error(`Could not download image: ${error.message}`);
  }
}

async function main() {
  const [eventId, outputPath] = process.argv.slice(2);
  
  if (!eventId || !outputPath) {
    console.error('Usage: node download-beeper-image.mjs <event_id> <output_path>');
    console.error('Example: node download-beeper-image.mjs "$abc:beeper.local" /tmp/image.jpg');
    process.exit(1);
  }
  
  try {
    console.log(`🔍 Looking up event: ${eventId}`);
    const attachment = await getImageInfo(eventId);
    
    console.log(`📸 Found image: ${attachment.fileName}`);
    console.log(`📏 Size: ${attachment.size.width}x${attachment.size.height}`);
    console.log(`💾 File size: ${attachment.fileSize} bytes`);
    
    await downloadImage(attachment.srcURL || attachment.id, outputPath);
    
  } catch (error) {
    console.error(`❌ Error: ${error.message}`);
    process.exit(1);
  }
}

main();
